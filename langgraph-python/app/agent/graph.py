"""Construction of the airline rebooking StateGraph."""

from typing import Any

from langgraph.graph import END, START, StateGraph

from app.agent.nodes import (
    ask_confirmation,
    evaluate_options,
    execute_rebooking,
    explain_invalid_change,
    get_reservation,
    get_travel_credits,
    search_alternatives,
    understand_request,
    validate_change,
    verify_rebooking,
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
    graph.add_node("refresh_context", _refresh_context)
    graph.add_node("evaluate_options", evaluate_options(llm))
    graph.add_node("ask_confirmation", ask_confirmation())
    graph.add_node("validate_change", validate_change(validator))
    graph.add_node("explain_invalid_change", explain_invalid_change())
    graph.add_node("execute_rebooking", execute_rebooking(airline))
    graph.add_node("verify_rebooking", verify_rebooking(airline))
    graph.add_node("finalize_failed", _finalize_failed)
    graph.add_node("finalize_declined", _finalize_declined)

    graph.add_edge(START, "understand_request")
    graph.add_edge("understand_request", "get_reservation")
    graph.add_edge("get_reservation", "search_alternatives")
    graph.add_edge("get_reservation", "get_travel_credits")
    graph.add_edge("refresh_context", "search_alternatives")
    graph.add_edge("refresh_context", "get_travel_credits")
    graph.add_edge(["search_alternatives", "get_travel_credits"], "evaluate_options")
    graph.add_edge("evaluate_options", "ask_confirmation")
    graph.add_edge("ask_confirmation", "validate_change")
    graph.add_conditional_edges("validate_change", _validation_route, {
        "valid": "execute_rebooking",
        "declined": "finalize_declined",
        "invalid": "explain_invalid_change",
    })
    graph.add_conditional_edges("explain_invalid_change", _explanation_route, {
        "retry": "refresh_context",
        "declined": "finalize_declined",
        "invalid": END,
    })
    graph.add_conditional_edges("execute_rebooking", _execution_route, {
        "completed": "verify_rebooking",
        "failed": "finalize_failed",
    })
    graph.add_conditional_edges("verify_rebooking", _execution_route, {
        "completed": END,
        "failed": "finalize_failed",
    })
    graph.add_edge("finalize_declined", END)
    graph.add_edge("finalize_failed", END)
    return graph.compile(checkpointer=checkpointer)


def _validation_route(state: AgentState) -> str:
    return state.get("route", "invalid")


def _explanation_route(state: AgentState) -> str:
    return state.get("route", "invalid")


def _execution_route(state: AgentState) -> str:
    return state.get("route", "completed")


def _refresh_context(_state: AgentState) -> dict[str, Any]:
    return {"route": "retry"}


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


def _finalize_failed(state: AgentState) -> dict[str, Any]:
    return {"route": "failed"}
