"""Nodes that load deterministic airline context."""

from app.agent.nodes.ports import AirlinePort, require_window
from app.agent.state import (
    AgentState,
    RebookingRequest,
    ReservationContext,
    SearchAlternativesResult,
    TravelCreditsResult,
)
from pydantic import ValidationError
from app.airline.schemas import ReservationDetailsSchema, SearchRebookingOptionsInputSchema


def _field(value, name: str):
    return value.get(name) if isinstance(value, dict) else getattr(value, name)


def _context(state: AgentState) -> ReservationContext:
    raw = state.get("reservation_context")
    if raw is None:
        raise ValueError("reservation context is required")
    raw_request = _field(raw, "request")
    raw_reservation = _field(raw, "reservation")
    request = (
        RebookingRequest.model_validate(raw_request)
        if isinstance(raw_request, dict)
        else raw_request
    )
    if isinstance(raw_reservation, dict):
        try:
            reservation = ReservationDetailsSchema.model_validate(raw_reservation)
        except ValidationError:
            reservation = ReservationDetailsSchema.model_construct(**raw_reservation)
    else:
        reservation = raw_reservation
    return ReservationContext.model_construct(request=request, reservation=reservation)


def get_reservation(airline: AirlinePort):
    async def node(state: AgentState) -> dict:
        request = state.get("request")
        if request is None:
            raise ValueError("reservation requires a request")
        reservation = await airline.get_reservation(request.booking_reference)
        if not reservation.segments:
            raise ValueError("reservation requires a segment")
        return {"reservation_context": ReservationContext(request=request, reservation=reservation)}

    return node


def _selected_segment(state: AgentState):
    context = _context(state)
    if context.request.segment_id is not None:
        for segment in _field(context.reservation, "segments"):
            if _field(_field(segment, "segment"), "id") == context.request.segment_id:
                return context, segment
        raise ValueError("requested reservation segment was not found")
    if len(_field(context.reservation, "segments")) != 1:
        raise ValueError("a reservation segment must be selected")
    return context, _field(context.reservation, "segments")[0]


def search_alternatives(airline: AirlinePort):
    async def node(state: AgentState) -> dict:
        context, segment = _selected_segment(state)
        departure_from, departure_to = require_window(context.request)
        options = await airline.search_rebooking_options(
            SearchRebookingOptionsInputSchema(
                segment_id=_field(_field(segment, "segment"), "id"),
                departure_from=departure_from,
                departure_to=departure_to,
                passenger_count=len(_field(context.reservation, "passengers")),
            )
        )
        return {"search_result": SearchAlternativesResult(request=context.request, options=options)}

    return node


def get_travel_credits(airline: AirlinePort):
    async def node(state: AgentState) -> dict:
        context = _context(state)
        reservation = _field(context.reservation, "reservation")
        credits = await airline.get_available_travel_credits(
            _field(reservation, "customer_id"),
            _field(reservation, "currency"),
        )
        return {"credits_result": TravelCreditsResult(request=context.request, credits=credits)}

    return node
