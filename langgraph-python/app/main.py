from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.config import load_settings
from app.platform.postgres import create_pool


settings = load_settings()


@asynccontextmanager
async def lifespan(application: FastAPI):
    pool = create_pool(settings)
    try:
        pool.open(wait=True)
        with pool.connection() as connection:
            connection.execute("SELECT 1")
        application.state.db = pool
        yield
    finally:
        pool.close()


application = FastAPI(title="Agents at Scale - LangGraph", lifespan=lifespan)


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
