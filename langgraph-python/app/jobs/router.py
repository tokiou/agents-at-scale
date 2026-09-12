from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.jobs.service import JobService

router = APIRouter(prefix="/jobs")


class JobRequest(BaseModel):
    payload: dict = Field(default_factory=dict)


@router.post("", status_code=202)
async def publish_job(job_request: JobRequest, request: Request):
    service: JobService = request.app.state.jobs
    try:
        job = await service.publish(job_request.payload)
    except Exception as error:
        raise HTTPException(status_code=500, detail="publish job failed") from error
    return {"id": job.id, "status": "published"}
