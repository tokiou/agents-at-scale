from datetime import datetime

import orjson
from redis.asyncio import BlockingConnectionPool, Redis

from app.config import Settings

STATUS_TTL_SECONDS = 24 * 60 * 60

# Deletes the lock only when it still holds our token, so an expired lock that
# another worker re-acquired is left alone.
_RELEASE_LOCK = """
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
"""


async def create_client(settings: Settings) -> Redis:
    # Bounded pool, the same cap as the Go runtime: without it redis-py opens a
    # new connection per concurrent command and can exhaust file descriptors.
    pool = BlockingConnectionPool.from_url(
        settings.redis_url,
        max_connections=settings.redis_pool_size,
        timeout=10,
        decode_responses=True,
    )
    client = Redis(connection_pool=pool)
    await client.ping()
    return client


class JobStore:
    """Job state and coordination data in Redis; same keys as the Go runtime."""

    def __init__(self, client: Redis) -> None:
        self._client = client
        self._release_lock = client.register_script(_RELEASE_LOCK)

    async def set_job_status(self, job_id: str, status: str) -> None:
        await self._client.set(f"job:{job_id}:status", status, ex=STATUS_TTL_SECONDS)

    async def get_job_status(self, job_id: str) -> str:
        return await self._client.get(f"job:{job_id}:status") or ""

    async def set_job_metadata(self, job_id: str, metadata: dict) -> None:
        """Store HITL resume data without changing the status-key contract."""
        await self._client.set(
            f"job:{job_id}:metadata",
            orjson.dumps(metadata, default=str),
            ex=STATUS_TTL_SECONDS,
        )

    async def mark_job_timing(self, job_id: str, field: str, at: datetime) -> None:
        """Record a timestamp in epoch ms, keeping only the first write so
        retries do not move published_ms or started_ms."""
        key = f"job:{job_id}:timing"
        async with self._client.pipeline(transaction=True) as pipe:
            pipe.hsetnx(key, field, int(at.timestamp() * 1000))
            pipe.expire(key, STATUS_TTL_SECONDS)
            await pipe.execute()

    async def set_job_timing(self, job_id: str, field: str, value: int) -> None:
        key = f"job:{job_id}:timing"
        async with self._client.pipeline(transaction=True) as pipe:
            pipe.hset(key, field, value)
            pipe.expire(key, STATUS_TTL_SECONDS)
            await pipe.execute()

    async def claim_idempotency_key(self, key: str, job_id: str) -> tuple[str, bool]:
        """Return the job id owning ``key`` and whether this call claimed it."""
        redis_key = f"airline:idempotency:{key}"
        if await self._client.set(redis_key, job_id, nx=True, ex=STATUS_TTL_SECONDS):
            return job_id, True
        return await self._client.get(redis_key), False

    async def release_idempotency_key(self, key: str) -> None:
        await self._client.delete(f"airline:idempotency:{key}")

    async def acquire_conversation_lock(self, conversation: str, token: str, ttl_ms: int) -> bool:
        return bool(await self._client.set(f"airline:lock:{conversation}", token, nx=True, px=ttl_ms))

    async def release_conversation_lock(self, conversation: str, token: str) -> None:
        await self._release_lock(keys=[f"airline:lock:{conversation}"], args=[token])
