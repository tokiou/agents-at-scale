"""Construction of the airline rebooking StateGraph."""

from typing import Any

from langgraph.graph import END, START, StateGraph

from app.agent.nodes import (
    ask_confirmation,
    evaluate_options,
    explain_invalid_change,
    get_reservation,
    get_travel_credits,
    search_alternatives,
    understand_request,
    validate_change,
)
from app.agent.nodes.ports import AgentLLM, AirlinePort, ChangeValidator
from app.agent.state import AgentState, FinalResult


def build_graph(
    llm: AgentLLM,
    airline: AirlinePort,
    validator: ChangeValidator,
    checkpointer: Any | None = None,
):
    """Build the graph; callers may supply memory or persistent checkpointing."""
    if validator is None:
        raise ValueError("a change validator is required")
    graph = StateGraph(AgentState)
    graph.add_node("understand_request", understand_request(llm))
    graph.add_node("get_reservation", get_reservation(airline))
    graph.add_node("search_alternatives", search_alternatives(airline))
    graph.add_node("get_travel_credits", get_travel_credits(airline))
    graph.add_node("evaluate_options", evaluate_options(llm))
    graph.add_node("ask_confirmation", ask_confirmation())
    graph.add_node("validate_change", validate_change(validator))
    graph.add_node("explain_invalid_change", explain_invalid_change())
    graph.add_node("finalize_ready", _finalize_ready)
    graph.add_node("finalize_declined", _finalize_declined)

    graph.add_edge(START, "understand_request")
    graph.add_edge("understand_request", "get_reservation")
    graph.add_edge("get_reservation", "search_alternatives")
    graph.add_edge("get_reservation", "get_travel_credits")
    graph.add_edge(["search_alternatives", "get_travel_credits"], "evaluate_options")
    graph.add_edge("evaluate_options", "ask_confirmation")
    graph.add_edge("ask_confirmation", "validate_change")
    graph.add_conditional_edges("validate_change", _validation_route, {
        "valid": "finalize_ready",
        "declined": "finalize_declined",
        "invalid": "explain_invalid_change",
    })
    graph.add_conditional_edges("explain_invalid_change", _explanation_route, {
        "retry": "evaluate_options",
        "declined": "finalize_declined",
        "invalid": END,
    })
    graph.add_edge("finalize_ready", END)
    graph.add_edge("finalize_declined", END)
    return graph.compile(checkpointer=checkpointer)


def _validation_route(state: AgentState) -> str:
    return state.get("route", "invalid")


def _explanation_route(state: AgentState) -> str:
    return state.get("route", "invalid")


def _finalize_ready(state: AgentState) -> dict[str, Any]:
    validation = state.get("validation")
    if validation is None or validation.selection is None:
        raise ValueError("ready result requires a validated selection")
    return {
        "final": FinalResult(
            status="ready",
            message="The rebooking selection is valid and ready for execution.",
            selection=validation.selection,
            validation=validation.validation,
        ),
        "route": "ready",
    }


def _finalize_declined(state: AgentState) -> dict[str, Any]:
    validation = state.get("validation")
    return {
        "final": FinalResult(
            status="declined",
            message="The rebooking was not confirmed.",
            selection=validation.selection if validation else None,
        ),
        "route": "declined",
    }
