"""Nodes that load deterministic airline context."""

from app.agent.nodes.ports import AirlinePort, require_window
from app.agent.state import (
    AgentState,
    ReservationContext,
    SearchAlternativesResult,
    TravelCreditsResult,
)
from app.airline.schemas import SearchRebookingOptionsInputSchema


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
    context = state.get("reservation_context")
    if context is None:
        raise ValueError("reservation context is required")
    if context.request.segment_id is not None:
        for segment in context.reservation.segments:
            if segment.segment.id == context.request.segment_id:
                return context, segment
        raise ValueError("requested reservation segment was not found")
    if len(context.reservation.segments) != 1:
        raise ValueError("a reservation segment must be selected")
    return context, context.reservation.segments[0]


def search_alternatives(airline: AirlinePort):
    async def node(state: AgentState) -> dict:
        context, segment = _selected_segment(state)
        departure_from, departure_to = require_window(context.request)
        options = await airline.search_rebooking_options(
            SearchRebookingOptionsInputSchema(
                segment_id=segment.segment.id,
                departure_from=departure_from,
                departure_to=departure_to,
                passenger_count=len(context.reservation.passengers),
            )
        )
        return {"search_result": SearchAlternativesResult(request=context.request, options=options)}

    return node


def get_travel_credits(airline: AirlinePort):
    async def node(state: AgentState) -> dict:
        context = state.get("reservation_context")
        if context is None:
            raise ValueError("reservation context is required")
        credits = await airline.get_available_travel_credits(
            context.reservation.reservation.customer_id,
            context.reservation.reservation.currency,
        )
        return {"credits_result": TravelCreditsResult(request=context.request, credits=credits)}

    return node
