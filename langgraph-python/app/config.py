import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    address: str
    database_url: str
    redis_url: str
    rabbitmq_url: str
    rabbitmq_queue: str
    max_open_conns: int
    max_idle_conns: int
    conn_max_lifetime_minutes: int


def load_settings() -> Settings:
    return Settings(
        address=os.getenv("AGENTS_SERVER_ADDR", "0.0.0.0:8080"),
        database_url=os.getenv(
            "DATABASE_URL",
            "postgresql://postgres:postgres@localhost:5432/agents_at_scale",
        ),
        redis_url=os.getenv("REDIS_URL", "redis://localhost:6379/0"),
        rabbitmq_url=os.getenv(
            "RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"
        ),
        rabbitmq_queue=os.getenv("RABBITMQ_QUEUE", "agent_jobs"),
        max_open_conns=_env_int("DB_MAX_OPEN_CONNS", 10, minimum=1),
        max_idle_conns=_env_int("DB_MAX_IDLE_CONNS", 5),
        conn_max_lifetime_minutes=_env_int("DB_CONN_MAX_LIFETIME_MINUTES", 30),
    )


def _env_int(name: str, default: int, minimum: int = 0) -> int:
    try:
        value = int(os.getenv(name, str(default)))
    except ValueError:
        return default
    return value if value >= minimum else default
