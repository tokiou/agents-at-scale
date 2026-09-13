from sqlalchemy.ext.asyncio import AsyncEngine, async_sessionmaker, create_async_engine

from app.config import Settings


def create_engine(settings: Settings) -> AsyncEngine:
    database_url = settings.database_url.replace("postgresql://", "postgresql+asyncpg://", 1)
    return create_async_engine(
        database_url,
        pool_size=settings.max_open_conns,
        max_overflow=0,
        pool_recycle=settings.conn_max_lifetime_minutes * 60,
        pool_pre_ping=True,
    )


def create_session_factory(engine: AsyncEngine):
    return async_sessionmaker(engine, expire_on_commit=False)
