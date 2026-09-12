from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.jobs import router as jobs_router
from app.jobs.service import JobService
from app.config import load_settings
from app.platform.postgres import create_pool
from app.platform.rabbitmq import RabbitMQ
from app.platform.redis import create_client

settings = load_settings()


@asynccontextmanager
async def lifespan(application: FastAPI):
    pool = create_pool(settings)
    redis = None
    rabbitmq = None
    job_service = None
    try:
        pool.open(wait=True)
        with pool.connection() as connection:
            connection.execute("SELECT 1")
        application.state.db = pool
        redis = await create_client(settings)
        rabbitmq = await RabbitMQ.connect(settings)
        application.state.redis = redis
        application.state.rabbitmq = rabbitmq
        job_service = JobService(redis, rabbitmq)
        await job_service.start()
        application.state.jobs = job_service
        yield
    finally:
        pool.close()
        if job_service is not None:
            await job_service.stop()
        if rabbitmq is not None:
            await rabbitmq.close()
        if redis is not None:
            await redis.aclose()


application = FastAPI(title="Agents at Scale - LangGraph", lifespan=lifespan)
application.include_router(jobs_router)


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
