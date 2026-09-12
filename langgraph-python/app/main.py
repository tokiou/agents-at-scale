from fastapi import FastAPI

from app.config import load_settings


settings = load_settings()
application = FastAPI(title="Agents at Scale - LangGraph")


@application.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
