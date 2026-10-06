import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from sqlalchemy import text

from app.jobs import router as jobs_router
from app.jobs.service import JobService, WorkerOptions
from app.agent.graph import build_graph
from app.agent.runner import AgentRunner
from app.airline.repository import customer, flight, flight_change, rebooking, reservation, travel_credit
from app.airline.repository.sql import customer as sql_customer
from app.airline.repository.sql import flight as sql_flight
from app.airline.repository.sql import flight_change as sql_flight_change
from app.airline.repository.sql import rebooking as sql_rebooking
from app.airline.repository.sql import reservation as sql_reservation
from app.airline.repository.sql import travel_credit as sql_travel_credit
from app.airline.service import Service
from app.platform.postgres import create_session_factory
from app.platform.llm import OpenRouterLLM
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool
from app.config import load_settings
from app.platform.postgres import create_engine, create_pool
from app.platform.rabbitmq import RabbitMQ
from app.platform.redis import JobStore, create_client

settings = load_settings()
logging.basicConfig(level=settings.log_level, format="%(asctime)s %(levelname)s %(name)s %(message)s")


@asynccontextmanager
async def lifespan(application: FastAPI):
    engine = None
    pg_pool = None
    redis = None
    rabbitmq = None
    job_service = None
    llm = None
    checkpoint_pool = None
    try:
        if settings.data_access == "asyncpg":
            pg_pool = await create_pool(settings)
            await pg_pool.fetchval("SELECT 1")
            source, modules = pg_pool, (
                sql_customer, sql_reservation, sql_flight, sql_travel_credit, sql_flight_change, sql_rebooking
            )
        else:
            engine = create_engine(settings)
            async with engine.begin() as connection:
                await connection.execute(text("SELECT 1"))
            source, modules = create_session_factory(engine), (
                customer, reservation, flight, travel_credit, flight_change, rebooking
            )
        customers, reservations, flights, credits, changes, rebookings = modules
        airline_service = Service(
            customers.CustomerRepository(source),
            reservations.ReservationRepository(source),
            flights.FlightRepository(source),
            credits.TravelCreditRepository(source),
            changes.FlightChangeRepository(source),
            rebookings.RebookingRepository(source),
        )
        logging.getLogger(__name__).info("airline data access: %s", settings.data_access)
        llm = OpenRouterLLM(settings)
        # A pool lets concurrent jobs checkpoint without sharing one connection.
        checkpoint_pool = AsyncConnectionPool(
            settings.database_url,
            max_size=settings.max_open_conns,
            kwargs={"autocommit": True, "prepare_threshold": 0, "row_factory": dict_row},
            open=False,
        )
        await checkpoint_pool.open()
        checkpointer = AsyncPostgresSaver(checkpoint_pool)
        if settings.run_migrations:
            await checkpointer.setup()
        graph = build_graph(
            llm,
            airline_service,
            airline_service,
            checkpointer,
        )
        application.state.agent = AgentRunner(graph)
        redis = await create_client(settings)
        rabbitmq = await RabbitMQ.connect(settings)
        application.state.redis = redis
        application.state.rabbitmq = rabbitmq
        job_service = JobService(
            JobStore(redis),
            rabbitmq,
            application.state.agent,
            WorkerOptions(
                concurrency=settings.worker_concurrency,
                max_attempts=settings.job_max_attempts,
                lock_ttl_ms=settings.job_lock_ttl_ms,
            ),
        )
        await job_service.start()
        application.state.jobs = job_service
        yield
    finally:
        if job_service is not None:
            await job_service.stop()
        if rabbitmq is not None:
            await rabbitmq.close()
        if llm is not None:
            await llm.close()
        if checkpoint_pool is not None:
            await checkpoint_pool.close()
        if redis is not None:
            await redis.aclose()
        if pg_pool is not None:
            await pg_pool.close()
        if engine is not None:
            await engine.dispose()


application = FastAPI(title="Agents at Scale - LangGraph", lifespan=lifespan)
application.include_router(jobs_router)


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
