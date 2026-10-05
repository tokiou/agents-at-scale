import json
import logging
from dataclasses import dataclass, replace
from datetime import UTC, datetime
from typing import Any, Protocol
from uuid import uuid4

from app.agent.runner import NoPendingInputError
from app.platform.rabbitmq import Job

logger = logging.getLogger(__name__)

# Job lifecycle statuses stored in Redis. Both runtimes use the same values.
STATUS_PUBLISHED = "published"
STATUS_PROCESSING = "processing"
STATUS_WAITING_FOR_CONFIRMATION = "waiting_for_confirmation"
STATUS_COMPLETED = "completed"
STATUS_FAILED = "failed"
STATUS_RETRYING = "retrying"
TERMINAL_STATUSES = {STATUS_COMPLETED, STATUS_WAITING_FOR_CONFIRMATION, STATUS_FAILED}

# Timing fields stored in job:<id>:timing as epoch milliseconds.
TIMING_PUBLISHED = "published_ms"
TIMING_STARTED = "started_ms"
TIMING_FINISHED = "finished_ms"
TIMING_ATTEMPTS = "attempts"


class JobStore(Protocol):
    async def set_job_status(self, job_id: str, status: str) -> None: ...
    async def get_job_status(self, job_id: str) -> str: ...
    async def set_job_metadata(self, job_id: str, metadata: dict) -> None: ...
    async def mark_job_timing(self, job_id: str, field: str, at: datetime) -> None: ...
    async def set_job_timing(self, job_id: str, field: str, value: int) -> None: ...
    async def claim_idempotency_key(self, key: str, job_id: str) -> tuple[str, bool]: ...
    async def release_idempotency_key(self, key: str) -> None: ...
    async def acquire_conversation_lock(self, conversation: str, token: str, ttl_ms: int) -> bool: ...
    async def release_conversation_lock(self, conversation: str, token: str) -> None: ...


class JobBroker(Protocol):
    async def publish(self, job: Job) -> None: ...
    async def retry(self, job: Job, level: int) -> None: ...
    async def dead_letter(self, job: Job) -> None: ...
    async def consume(self, callback, prefetch: int) -> str: ...
    async def cancel_consumer(self, consumer_tag: str) -> None: ...


@dataclass(frozen=True)
class WorkerOptions:
    concurrency: int = 1
    max_attempts: int = 3
    lock_ttl_ms: int = 300_000


class InvalidJobError(ValueError):
    """The job can never succeed; it is dead-lettered without retries."""


def validate_request(payload: Any) -> dict[str, Any]:
    """Validate a queued chat payload; mirrors the Go worker validation."""
    if not isinstance(payload, dict):
        raise InvalidJobError("payload must be an object")
    for field in ("user_id", "session_id"):
        value = payload.get(field)
        if not isinstance(value, str) or not value.strip():
            raise InvalidJobError(f"{field} is required")
    message = payload.get("message")
    has_message = isinstance(message, str) and bool(message.strip())
    resume = payload.get("resume")
    if has_message == (resume is not None):
        raise InvalidJobError("exactly one of message or resume is required")
    if resume is not None:
        if not isinstance(resume, dict):
            raise InvalidJobError("resume must be an object")
        if not isinstance(resume.get("confirmed"), bool):
            raise InvalidJobError("resume.confirmed is required")
        if resume["confirmed"] and not resume.get("selection"):
            raise InvalidJobError("resume.selection is required when confirmed")
    return payload


def _now() -> datetime:
    return datetime.now(UTC)


def _now_ms() -> int:
    return int(_now().timestamp() * 1000)


class JobService:
    def __init__(self, store: JobStore, broker: JobBroker, runner=None, options: WorkerOptions | None = None) -> None:
        self._store = store
        self._broker = broker
        self._runner = runner
        self._options = options or WorkerOptions()
        self._consumer_tag: str | None = None

    async def start(self) -> None:
        """Consume up to ``concurrency`` jobs at once; zero only publishes."""
        if self._options.concurrency <= 0:
            logger.info("job consumer disabled")
            return
        self._consumer_tag = await self._broker.consume(self.consume, self._options.concurrency)
        logger.info(
            "job consumer started concurrency=%d max_attempts=%d",
            self._options.concurrency,
            self._options.max_attempts,
        )

    async def stop(self) -> None:
        if self._consumer_tag is not None:
            await self._broker.cancel_consumer(self._consumer_tag)
            self._consumer_tag = None

    async def publish(self, payload: dict, idempotency_key: str | None = None) -> tuple[str, bool]:
        """Queue a chat payload. A repeated ``idempotency_key`` returns the
        original job id with ``duplicate=True`` and queues nothing."""
        job = Job(id=str(uuid4()), payload=payload)
        if idempotency_key:
            owner, claimed = await self._store.claim_idempotency_key(idempotency_key, job.id)
            if not claimed:
                return owner, True
        try:
            await self._store.set_job_status(job.id, STATUS_PUBLISHED)
            await self._store.mark_job_timing(job.id, TIMING_PUBLISHED, _now())
            try:
                await self._broker.publish(job)
            except Exception:
                await self._store.set_job_status(job.id, STATUS_FAILED)
                raise
        except Exception:
            if idempotency_key:
                await self._store.release_idempotency_key(idempotency_key)
            raise
        return job.id, False

    async def consume(self, message) -> None:
        try:
            data = json.loads(message.body.decode())
            job = Job(**data)
            if not job.id:
                raise ValueError("job id is required")
        except Exception:
            logger.exception("invalid job envelope")
            await message.nack(requeue=False)
            return
        try:
            await self._process(message, job)
        except Exception as error:
            # Infrastructure failures (Redis, broker) outside the agent run.
            await self._retry(message, job, "job processing failed", error)

    async def _process(self, message, job: Job) -> None:
        # RabbitMQ delivers at least once; a job that already reached a final
        # state is a duplicate delivery and must not run the agent again.
        status = await self._store.get_job_status(job.id)
        if status in TERMINAL_STATUSES:
            logger.warning("duplicate job delivery skipped job_id=%s status=%s", job.id, status)
            await message.ack()
            return
        try:
            request = validate_request(job.payload)
            if self._runner is None:
                raise InvalidJobError("agent runner is required")
        except InvalidJobError as error:
            await self._dead_letter(message, job, "invalid job payload", error)
            return

        # One job per conversation at a time: a second job for the same
        # session waits in the first retry queue without consuming an attempt.
        user_id, session_id = request["user_id"], request["session_id"]
        conversation = f"{user_id}:{session_id}"
        if not await self._store.acquire_conversation_lock(conversation, job.id, self._options.lock_ttl_ms):
            logger.info("conversation busy, delaying job job_id=%s conversation=%s", job.id, conversation)
            await self._delay(message, job, 1)
            return
        try:
            await self._store.set_job_status(job.id, STATUS_PROCESSING)
            await self._store.mark_job_timing(job.id, TIMING_STARTED, _now())
            await self._store.set_job_timing(job.id, TIMING_ATTEMPTS, job.attempt + 1)
            logger.info(
                "job consumed job_id=%s user_id=%s session_id=%s attempt=%d",
                job.id,
                user_id,
                session_id,
                job.attempt + 1,
            )
            try:
                if "resume" in request:
                    result = await self._runner.resume(user_id, session_id, request["resume"])
                else:
                    result = await self._runner.run(user_id, session_id, request["message"])
            except NoPendingInputError as error:
                await self._dead_letter(message, job, "agent run failed", error)
                return
            except Exception as error:
                await self._retry(message, job, "agent run failed", error)
                return
            await self._finish(message, job, user_id, session_id, result)
        finally:
            try:
                await self._store.release_conversation_lock(conversation, job.id)
            except Exception:
                logger.exception("release conversation lock failed job_id=%s", job.id)

    async def _finish(self, message, job: Job, user_id: str, session_id: str, result: dict) -> None:
        if "__interrupt__" in result:
            interrupt = result["__interrupt__"][0]
            value = getattr(interrupt, "value", interrupt)
            value = value if isinstance(value, dict) else {}
            await self._store.set_job_metadata(
                job.id,
                {
                    "user_id": user_id,
                    "session_id": session_id,
                    "message": value.get("message", ""),
                    "evaluation": value.get("evaluation"),
                },
            )
            status = STATUS_WAITING_FOR_CONFIRMATION
        else:
            final = result.get("final")
            final_status = final.get("status") if isinstance(final, dict) else getattr(final, "status", None)
            if final_status == "failed":
                reason = final.get("message") if isinstance(final, dict) else getattr(final, "message", "")
                await self._dead_letter(message, job, "agent finished with failure", RuntimeError(reason))
                return
            status = STATUS_COMPLETED
        await self._store.set_job_timing(job.id, TIMING_FINISHED, _now_ms())
        await self._store.set_job_status(job.id, status)
        await message.ack()

    async def _retry(self, message, job: Job, reason: str, error: Exception) -> None:
        """Schedule another attempt with exponential backoff, or dead-letter
        the job when its attempts are exhausted."""
        job = replace(job, attempt=job.attempt + 1)
        if job.attempt >= self._options.max_attempts:
            await self._dead_letter(message, job, reason, error)
            return
        logger.warning("%s, retrying job_id=%s attempt=%d error=%s", reason, job.id, job.attempt, error)
        try:
            await self._store.set_job_status(job.id, STATUS_RETRYING)
        except Exception:
            logger.exception("set retrying status failed job_id=%s", job.id)
        await self._delay(message, job, job.attempt)

    async def _delay(self, message, job: Job, level: int) -> None:
        try:
            await self._broker.retry(job, level)
        except Exception:
            logger.exception("schedule retry failed, requeueing job_id=%s", job.id)
            await message.nack(requeue=True)
            return
        await message.ack()

    async def _dead_letter(self, message, job: Job, reason: str, error: Exception) -> None:
        logger.error("%s job_id=%s attempt=%d error=%s", reason, job.id, job.attempt, error)
        job = replace(job, error=str(error))
        try:
            await self._store.set_job_status(job.id, STATUS_FAILED)
            await self._store.set_job_timing(job.id, TIMING_FINISHED, _now_ms())
        except Exception:
            logger.exception("set failed status failed job_id=%s", job.id)
        try:
            await self._broker.dead_letter(job)
        except Exception:
            logger.exception("dead-letter job failed job_id=%s", job.id)
            await message.nack(requeue=False)
            return
        await message.ack()
