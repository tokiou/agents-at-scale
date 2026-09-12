import json
from collections.abc import Awaitable, Callable
from dataclasses import dataclass

import aio_pika
from aio_pika import DeliveryMode, IncomingMessage, Message

from app.config import Settings


@dataclass(frozen=True)
class Job:
    id: str
    payload: dict


class RabbitMQ:
    def __init__(self, connection, channel, queue) -> None:
        self._connection = connection
        self._channel = channel
        self._queue = queue

    @classmethod
    async def connect(cls, settings: Settings) -> "RabbitMQ":
        connection = await aio_pika.connect_robust(settings.rabbitmq_url)
        channel = await connection.channel()
        await channel.set_qos(prefetch_count=1)
        queue = await channel.declare_queue(settings.rabbitmq_queue, durable=True)
        return cls(connection, channel, queue)

    async def publish(self, job: Job) -> None:
        message = Message(
            json.dumps({"id": job.id, "payload": job.payload}).encode(),
            content_type="application/json",
            delivery_mode=DeliveryMode.PERSISTENT,
        )
        await self._channel.default_exchange.publish(
            message,
            routing_key=self._queue.name,
        )

    async def consume(
        self,
        callback: Callable[[IncomingMessage], Awaitable[None]],
    ) -> str:
        return await self._queue.consume(callback)

    async def cancel_consumer(self, consumer_tag: str) -> None:
        await self._queue.cancel(consumer_tag)

    async def close(self) -> None:
        await self._connection.close()
