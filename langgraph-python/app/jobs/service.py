import json
import logging
from uuid import uuid4

from app.platform.rabbitmq import Job, RabbitMQ
from app.platform.redis import set_job_status

logger = logging.getLogger(__name__)


class JobService:
    def __init__(self, redis, rabbitmq: RabbitMQ, runner=None) -> None:
        self._redis = redis
        self._rabbitmq = rabbitmq
        self._consumer_tag: str | None = None
        self._runner = runner

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
            if self._runner is None:
                raise RuntimeError("agent runner is required")
            result = await self._runner.run(job.payload)
            if "__interrupt__" in result:
                status = "waiting_for_confirmation"
            else:
                final = result.get("final")
                final_status = (
                    final.get("status") if isinstance(final, dict) else getattr(final, "status", None)
                )
                status = "failed" if final_status == "failed" else "completed"
            await set_job_status(self._redis, job.id, status)
            await message.ack()
        except Exception:
            logger.exception("job processing failed")
            try:
                await set_job_status(self._redis, job.id, "failed")
            except Exception:
                logger.exception("failed to update job status")
            await message.nack(requeue=False)
