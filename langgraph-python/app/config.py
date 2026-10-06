import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    address: str
    log_level: str
    database_url: str
    redis_url: str
    redis_pool_size: int
    rabbitmq_url: str
    rabbitmq_queue: str
    worker_concurrency: int
    data_access: str
    job_max_attempts: int
    job_retry_base_ms: int
    job_lock_ttl_ms: int
    run_migrations: bool
    llm_max_connections: int
    llm_timeout_seconds: int
    max_open_conns: int
    max_idle_conns: int
    conn_max_lifetime_minutes: int
    openrouter_deployment: str
    openrouter_api_key: str
    openrouter_base_url: str

    def retry_delays_ms(self) -> list[int]:
        """One exponential backoff delay per retry level: base, 2*base...
        for JOB_MAX_ATTEMPTS-1 retries (at least one level, which also delays
        jobs whose conversation is busy). Mirrors Go Config.RetryDelays."""
        levels = max(self.job_max_attempts - 1, 1)
        return [self.job_retry_base_ms << level for level in range(levels)]


def load_settings() -> Settings:
    return Settings(
        address=os.getenv("AGENTS_SERVER_ADDR", "0.0.0.0:8080"),
        log_level=os.getenv("LOG_LEVEL", "info").upper(),
        database_url=os.getenv(
            "DATABASE_URL",
            "postgresql://postgres:postgres@localhost:5432/agents_at_scale",
        ),
        redis_url=os.getenv("REDIS_URL", "redis://localhost:6379/0"),
        redis_pool_size=_env_int("REDIS_POOL_SIZE", 50, minimum=1),
        rabbitmq_url=os.getenv(
            "RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"
        ),
        rabbitmq_queue=os.getenv("RABBITMQ_QUEUE", "agent_jobs"),
        worker_concurrency=_env_int("WORKER_CONCURRENCY", 1),
        data_access=_data_access(os.getenv("DATA_ACCESS", "orm")),
        job_max_attempts=_env_int("JOB_MAX_ATTEMPTS", 3),
        job_retry_base_ms=_env_int("JOB_RETRY_BASE_MS", 1000),
        job_lock_ttl_ms=_env_int("JOB_LOCK_TTL_MS", 300_000),
        run_migrations=os.getenv("RUN_MIGRATIONS", "true") == "true",
        llm_max_connections=_env_int("LLM_MAX_CONNECTIONS", 100, minimum=1),
        llm_timeout_seconds=_env_int("LLM_TIMEOUT_SECONDS", 60, minimum=1),
        max_open_conns=_env_int("DB_MAX_OPEN_CONNS", 10, minimum=1),
        max_idle_conns=_env_int("DB_MAX_IDLE_CONNS", 5),
        conn_max_lifetime_minutes=_env_int("DB_CONN_MAX_LIFETIME_MINUTES", 30),
        openrouter_deployment=os.getenv("OPENROUTER_DEPLOYMENT", ""),
        openrouter_api_key=os.getenv("OPENROUTER_API_KEY", ""),
        openrouter_base_url=os.getenv(
            "OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"
        ),
    )


def _env_int(name: str, default: int, minimum: int = 0) -> int:
    try:
        value = int(os.getenv(name, str(default)))
    except ValueError:
        return default
    return value if value >= minimum else default


def _data_access(value: str) -> str:
    """orm (SQLAlchemy ORM, default) or asyncpg (hand-written SQL that mirrors
    the Go runtime's sqlc queries)."""
    if value not in ("orm", "asyncpg"):
        raise ValueError(f"DATA_ACCESS must be orm or asyncpg, got {value!r}")
    return value
