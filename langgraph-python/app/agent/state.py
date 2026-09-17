"""Contracts shared by the airline rebooking graph."""

from datetime import datetime
from decimal import Decimal
from typing import Any, Literal, TypedDict
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field

from app.airline.schemas import (
    RebookingOptionSchema,
    ReservationDetailsSchema,
    TravelCreditSchema,
)


class RebookingRequest(BaseModel):
    booking_reference: str
    segment_id: UUID | None = None
    departure_from: datetime | None = None
    departure_to: datetime | None = None


class ReservationContext(BaseModel):
    request: RebookingRequest
    reservation: ReservationDetailsSchema


class SearchAlternativesResult(BaseModel):
    request: RebookingRequest
    options: list[RebookingOptionSchema]


class TravelCreditsResult(BaseModel):
    request: RebookingRequest
    credits: list[TravelCreditSchema]


class RebookingSelection(BaseModel):
    segment_id: UUID
    new_flight_id: UUID
    new_fare_class_id: UUID
    travel_credit_id: UUID | None = None
    travel_credit_amount: Decimal = Decimal("0")


class EvaluationResult(BaseModel):
    request: RebookingRequest
    options: list[RebookingOptionSchema]
    credits: list[TravelCreditSchema]
    ranked_option_ids: list[UUID] = Field(default_factory=list)
    summary: str = ""
    has_matching_option: bool = False


class ConfirmationResult(BaseModel):
    confirmed: bool
    selection: RebookingSelection | None = None
    evaluation: EvaluationResult


class ValidationResult(BaseModel):
    valid: bool = False
    selection: RebookingSelection | None = None
    reason: str = ""
    evaluation: EvaluationResult
    user_declined: bool = False
    validation: Any | None = None


class FinalResult(BaseModel):
    status: Literal["ready", "declined", "failed"]
    message: str
    selection: RebookingSelection | None = None
    validation: Any | None = None


class AgentState(TypedDict, total=False):
    user_input: str
    request: RebookingRequest
    reservation_context: ReservationContext
    search_result: SearchAlternativesResult
    credits_result: TravelCreditsResult
    evaluation: EvaluationResult
    confirmation: ConfirmationResult
    validation: ValidationResult
    final: FinalResult
    route: Literal["valid", "invalid", "declined", "retry", "ready"]
    error: str
    retry_count: int
    max_retries: int


def state_to_dict(state: AgentState) -> dict[str, Any]:
    """Return a JSON-compatible copy useful at graph boundaries."""
    return {key: value.model_dump(mode="json") if isinstance(value, BaseModel) else value for key, value in state.items()}
