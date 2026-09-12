from psycopg_pool import ConnectionPool

from app.config import Settings


def create_pool(settings: Settings) -> ConnectionPool:
    min_size = min(settings.max_idle_conns, settings.max_open_conns)
    return ConnectionPool(
        conninfo=settings.database_url,
        min_size=min_size,
        max_size=settings.max_open_conns,
        max_lifetime=settings.conn_max_lifetime_minutes * 60,
        open=False,
    )
