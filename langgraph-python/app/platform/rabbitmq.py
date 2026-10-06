from collections.abc import Awaitable, Callable
from dataclasses import asdict, dataclass

import aio_pika
import orjson
from aio_pika import DeliveryMode, IncomingMessage, Message

from app.config import Settings


@dataclass(frozen=True)
class Job:
    """Queued envelope. ``attempt`` counts failed processing attempts and
    ``error`` keeps the last failure when the job is dead-lettered."""

    id: str
    payload: dict
    attempt: int = 0
    error: str | None = None


def retry_queue(queue: str, level: int) -> str:
    return f"{queue}.retry.{level}"


def dead_queue(queue: str) -> str:
    return f"{queue}.dead"


class RabbitMQ:
    """Job broker. Besides the main queue it declares one delay queue per
    retry level whose messages expire back into the main queue, and a dead
    queue for jobs that will not be retried. Mirrors the Go client."""

    def __init__(self, connection, channel, queue, retry_queues: list[str]) -> None:
        self._connection = connection
        self._channel = channel
        self._queue = queue
        self._retry_queues = retry_queues

    @classmethod
    async def connect(cls, settings: Settings) -> "RabbitMQ":
        connection = await aio_pika.connect_robust(settings.rabbitmq_url)
        channel = await connection.channel()
        queue = await channel.declare_queue(settings.rabbitmq_queue, durable=True)
        await channel.declare_queue(dead_queue(settings.rabbitmq_queue), durable=True)
        retry_queues = []
        for level, delay_ms in enumerate(settings.retry_delays_ms(), start=1):
            name = retry_queue(settings.rabbitmq_queue, level)
            await channel.declare_queue(
                name,
                durable=True,
                arguments={
                    "x-message-ttl": delay_ms,
                    "x-dead-letter-exchange": "",
                    "x-dead-letter-routing-key": settings.rabbitmq_queue,
                },
            )
            retry_queues.append(name)
        return cls(connection, channel, queue, retry_queues)

    async def publish(self, job: Job) -> None:
        await self._publish(self._queue.name, job)

    async def retry(self, job: Job, level: int) -> None:
        """Publish to the delay queue of ``level`` (1-based); levels beyond
        the declared ones use the longest delay."""
        if not self._retry_queues:
            raise RuntimeError("no retry queues declared")
        level = min(max(level, 1), len(self._retry_queues))
        await self._publish(self._retry_queues[level - 1], job)

    async def dead_letter(self, job: Job) -> None:
        await self._publish(dead_queue(self._queue.name), job)

    async def _publish(self, routing_key: str, job: Job) -> None:
        body = {key: value for key, value in asdict(job).items() if value is not None}
        message = Message(
            orjson.dumps(body),
            content_type="application/json",
            delivery_mode=DeliveryMode.PERSISTENT,
        )
        await self._channel.default_exchange.publish(message, routing_key=routing_key)

    async def consume(
        self,
        callback: Callable[[IncomingMessage], Awaitable[None]],
        prefetch: int,
    ) -> str:
        """Run callbacks concurrently with at most ``prefetch`` unacked jobs."""
        await self._channel.set_qos(prefetch_count=prefetch)
        return await self._queue.consume(callback)

    async def cancel_consumer(self, consumer_tag: str) -> None:
        await self._queue.cancel(consumer_tag)

    async def close(self) -> None:
        await self._connection.close()
