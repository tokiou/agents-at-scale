"""Dependency ports used by agent nodes."""

from datetime import datetime
from typing import Any, Protocol
from uuid import UUID

from app.agent.state import (
    EvaluationResult,
    RebookingRequest,
    RebookingSelection,
    ValidationResult,
)
from app.airline.schemas import (
    RebookingOptionSchema,
    ReservationDetailsSchema,
    SearchRebookingOptionsInputSchema,
    TravelCreditSchema,
)


class AgentLLM(Protocol):
    async def understand_request(self, user_input: str) -> RebookingRequest: ...

    async def evaluate_options(
        self,
        request: RebookingRequest,
        options: list[RebookingOptionSchema],
        credits: list[TravelCreditSchema],
    ) -> EvaluationResult: ...


class AirlinePort(Protocol):
    async def get_reservation(self, booking_reference: str) -> ReservationDetailsSchema: ...

    async def search_rebooking_options(
        self, input_data: SearchRebookingOptionsInputSchema
    ) -> list[RebookingOptionSchema]: ...

    async def get_available_travel_credits(
        self, customer_id: UUID, currency: str
    ) -> list[TravelCreditSchema]: ...


class ChangeValidator(Protocol):
    async def validate_change(self, selection: RebookingSelection) -> Any: ...


def require_window(request: RebookingRequest) -> tuple[datetime, datetime]:
    if request.departure_from is None or request.departure_to is None:
        raise ValueError("departure window is required")
    if request.departure_to < request.departure_from:
        raise ValueError("departure window is invalid")
    return request.departure_from, request.departure_to
