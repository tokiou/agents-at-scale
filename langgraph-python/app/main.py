from contextlib import asynccontextmanager

from fastapi import FastAPI
from sqlalchemy import text

from app.jobs import router as jobs_router
from app.jobs.service import JobService
from app.agent.graph import build_graph
from app.agent.runner import AgentRunner
from app.airline.repository.customer import CustomerRepository
from app.airline.repository.flight import FlightRepository
from app.airline.repository.flight_change import FlightChangeRepository
from app.airline.repository.rebooking import RebookingRepository
from app.airline.repository.reservation import ReservationRepository
from app.airline.repository.travel_credit import TravelCreditRepository
from app.airline.service import Service
from app.platform.postgres import create_session_factory
from app.platform.llm import OpenRouterLLM
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from app.config import load_settings
from app.platform.postgres import create_engine
from app.platform.rabbitmq import RabbitMQ
from app.platform.redis import create_client

settings = load_settings()


@asynccontextmanager
async def lifespan(application: FastAPI):
    engine = create_engine(settings)
    redis = None
    rabbitmq = None
    job_service = None
    llm = None
    checkpointer_context = None
    checkpointer_started = False
    try:
        async with engine.begin() as connection:
            await connection.execute(text("SELECT 1"))
        application.state.db = engine
        sessions = create_session_factory(engine)
        rebooking = RebookingRepository(sessions)
        airline_service = Service(
            CustomerRepository(sessions),
            ReservationRepository(sessions),
            FlightRepository(sessions),
            TravelCreditRepository(sessions),
            FlightChangeRepository(sessions),
            rebooking,
        )
        llm = OpenRouterLLM(settings)
        checkpointer_context = AsyncPostgresSaver.from_conn_string(settings.database_url)
        checkpointer = await checkpointer_context.__aenter__()
        checkpointer_started = True
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
        job_service = JobService(redis, rabbitmq, application.state.agent)
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
        if checkpointer_started:
            await checkpointer_context.__aexit__(None, None, None)
        if redis is not None:
            await redis.aclose()
        await engine.dispose()


application = FastAPI(title="Agents at Scale - LangGraph", lifespan=lifespan)
application.include_router(jobs_router)


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
