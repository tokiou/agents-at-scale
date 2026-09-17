from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.jobs.service import JobService

router = APIRouter(prefix="/jobs")


class JobRequest(BaseModel):
    payload: dict = Field(default_factory=dict)

    @property
    def thread_id(self) -> str | None:
        value = self.payload.get("thread_id")
        return value if isinstance(value, str) and value else None


@router.post("", status_code=202)
async def publish_job(job_request: JobRequest, request: Request):
    if job_request.thread_id is None:
        raise HTTPException(status_code=422, detail="payload.thread_id is required")
    if "input" not in job_request.payload and "resume" not in job_request.payload:
        raise HTTPException(status_code=422, detail="payload.input or payload.resume is required")
    service: JobService = request.app.state.jobs
    try:
        job = await service.publish(job_request.payload)
    except Exception as error:
        raise HTTPException(status_code=500, detail="publish job failed") from error
    return {"id": job.id, "status": "published"}
