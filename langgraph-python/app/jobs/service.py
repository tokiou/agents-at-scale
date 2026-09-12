import json
import logging
from uuid import uuid4

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.platform.rabbitmq import Job, RabbitMQ
from app.platform.redis import set_job_status

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/jobs")


class JobRequest(BaseModel):
    payload: dict = Field(default_factory=dict)


class JobService:
    def __init__(self, redis, rabbitmq: RabbitMQ) -> None:
        self._redis = redis
        self._rabbitmq = rabbitmq
        self._consumer_tag: str | None = None

    async def start(self) -> None:
        self._consumer_tag = await self._rabbitmq.consume(self.consume)

    async def stop(self) -> None:
        if self._consumer_tag is not None:
            await self._rabbitmq.cancel_consumer(self._consumer_tag)
            self._consumer_tag = None

    async def publish(self, payload: dict) -> Job:
        job = Job(id=str(uuid4()), payload=payload)
        await set_job_status(self._redis, job.id, "published")
        try:
            await self._rabbitmq.publish(job)
        except Exception:
            await set_job_status(self._redis, job.id, "failed")
            raise
        return job

    async def consume(self, message) -> None:
        try:
            job = Job(**json.loads(message.body.decode()))
            await set_job_status(self._redis, job.id, "processing")
            logger.info("job consumed id=%s payload=%s", job.id, job.payload)
            await set_job_status(self._redis, job.id, "completed")
            await message.ack()
        except Exception:
            logger.exception("job processing failed")
            await message.nack(requeue=False)


@router.post("", status_code=202)
async def publish_job(job_request: JobRequest, request: Request):
    service: JobService = request.app.state.jobs
    try:
        job = await service.publish(job_request.payload)
    except Exception as error:
        raise HTTPException(status_code=500, detail="publish job failed") from error
    return {"id": job.id, "status": "published"}
