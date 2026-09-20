from redis.asyncio import Redis

from app.config import Settings

STATUS_TTL_SECONDS = 24 * 60 * 60


async def create_client(settings: Settings) -> Redis:
    client = Redis.from_url(settings.redis_url, decode_responses=True)
    await client.ping()
    return client


async def set_job_status(client: Redis, job_id: str, status: str) -> None:
    await client.set(
        f"job:{job_id}:status",
        status,
        ex=STATUS_TTL_SECONDS,
    )


async def set_job_metadata(client: Redis, job_id: str, metadata: dict) -> None:
    """Store HITL resume data without changing the status-key contract."""
    import json

    await client.set(
        f"job:{job_id}:metadata",
        json.dumps(metadata, default=str),
        ex=STATUS_TTL_SECONDS,
    )
