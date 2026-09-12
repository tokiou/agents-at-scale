import json
import logging
from contextlib import asynccontextmanager
from uuid import uuid4

from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel, Field

from app.config import load_settings
from app.platform.postgres import create_pool
from app.platform.rabbitmq import Job, RabbitMQ
from app.platform.redis import create_client, set_job_status

logger = logging.getLogger(__name__)

settings = load_settings()


@asynccontextmanager
async def lifespan(application: FastAPI):
    pool = create_pool(settings)
    redis = None
    rabbitmq = None
    consumer_tag = None
    try:
        pool.open(wait=True)
        with pool.connection() as connection:
            connection.execute("SELECT 1")
        application.state.db = pool
        redis = await create_client(settings)
        rabbitmq = await RabbitMQ.connect(settings)
        application.state.redis = redis
        application.state.rabbitmq = rabbitmq
        consumer_tag = await rabbitmq.consume(consume_job)
        yield
    finally:
        pool.close()
        if rabbitmq is not None:
            await rabbitmq.close(consumer_tag)
        if redis is not None:
            await redis.aclose()


application = FastAPI(title="Agents at Scale - LangGraph", lifespan=lifespan)


class JobRequest(BaseModel):
    payload: dict = Field(default_factory=dict)


async def consume_job(message) -> None:
    try:
        job = Job(**json.loads(message.body.decode()))
        await set_job_status(application.state.redis, job.id, "processing")
        logger.info("job consumed id=%s payload=%s", job.id, job.payload)
        await set_job_status(application.state.redis, job.id, "completed")
        await message.ack()
    except Exception:
        logger.exception("job processing failed")
        await message.nack(requeue=False)


@application.post("/jobs", status_code=202)
async def publish_job(job_request: JobRequest, request: Request):
    job = Job(id=str(uuid4()), payload=job_request.payload)
    try:
        await set_job_status(request.app.state.redis, job.id, "published")
        await request.app.state.rabbitmq.publish(job)
    except Exception as error:
        await set_job_status(request.app.state.redis, job.id, "failed")
        raise HTTPException(status_code=500, detail="publish job failed") from error
    return {"id": job.id, "status": "published"}


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
