import logging
from typing import Any

from fastapi import APIRouter, Body, Header, HTTPException, Request
from pydantic import BaseModel, ValidationError, model_validator

from app.airline.schemas import RebookingSelectionSchema
from app.jobs.service import JobService

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/airline/chat")


class ResumeRequest(BaseModel):
    confirmed: bool
    selection: RebookingSelectionSchema | None = None

    @model_validator(mode="after")
    def require_selection(self) -> "ResumeRequest":
        if self.confirmed and self.selection is None:
            raise ValueError("resume.selection is required when confirmed")
        return self


class ChatRequest(BaseModel):
    """Public chat contract shared with the Go runtime.

    A start request sends ``message``; a confirmation answer sends ``resume``.
    """

    user_id: str
    session_id: str
    message: str | None = None
    resume: ResumeRequest | None = None

    @model_validator(mode="after")
    def require_one_action(self) -> "ChatRequest":
        if not self.user_id.strip() or not self.session_id.strip():
            raise ValueError("user_id and session_id are required")
        has_message = bool(self.message and self.message.strip())
        if has_message == (self.resume is not None):
            raise ValueError("exactly one of message or resume is required")
        return self


@router.post("", status_code=202)
async def publish_chat(
    request: Request,
    body: Any = Body(...),
    idempotency_key: str | None = Header(default=None, alias="Idempotency-Key"),
) -> dict[str, str]:
    """Queue a chat job. A repeated Idempotency-Key returns the original job
    id with status "duplicate" instead of queueing it again."""
    # Validation errors use the same {"detail": "<message>"} body as Go.
    try:
        chat = ChatRequest.model_validate(body)
    except ValidationError as error:
        first = error.errors()[0]
        detail = first["msg"].removeprefix("Value error, ")
        if first["type"] == "missing":
            detail = f"{'.'.join(str(part) for part in first['loc'])} is required"
        raise HTTPException(status_code=422, detail=detail) from error
    key = (idempotency_key or "").strip()
    if len(key) > 200:
        raise HTTPException(status_code=422, detail="Idempotency-Key is too long")
    service: JobService = request.app.state.jobs
    try:
        job_id, duplicate = await service.publish(chat.model_dump(mode="json", exclude_none=True), key or None)
    except Exception as error:
        logger.exception("publish job failed")
        raise HTTPException(status_code=500, detail="publish job failed") from error
    return {"id": job_id, "status": "duplicate" if duplicate else "published"}
