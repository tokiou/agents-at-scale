"""Language-model nodes."""

from app.agent.state import (
    AgentState,
    EvaluationResult,
    RebookingRequest,
    SearchAlternativesResult,
    TravelCreditsResult,
)
from app.agent.nodes.ports import AgentLLM


def _field(value, name: str):
    return value.get(name) if isinstance(value, dict) else getattr(value, name)


def understand_request(llm: AgentLLM):
    async def node(state: AgentState) -> dict:
        user_input = state.get("user_input", "").strip()
        if not user_input:
            raise ValueError("user input is required")
        request = await llm.understand_request(user_input)
        if not request.booking_reference:
            raise ValueError("booking reference is required")
        return {"request": request}

    return node


def evaluate_options(llm: AgentLLM):
    async def node(state: AgentState) -> dict:
        raw_request = state.get("request")
        raw_search = state.get("search_result")
        raw_credits = state.get("credits_result")
        request = RebookingRequest.model_validate(raw_request) if isinstance(raw_request, dict) else raw_request
        search = (
            SearchAlternativesResult.model_validate(raw_search)
            if isinstance(raw_search, dict)
            else raw_search
        )
        credits = (
            TravelCreditsResult.model_validate(raw_credits)
            if isinstance(raw_credits, dict)
            else raw_credits
        )
        if request is None or search is None or credits is None:
            raise ValueError("evaluation requires request, alternatives, and credits")
        result = await llm.evaluate_options(request, search.options, credits.credits)
        allowed_ids = {
            _field(_field(option, "flight"), "id") for option in search.options
        } | {
            _field(_field(option, "flight_fare"), "id") for option in search.options
        }
        if any(option_id not in allowed_ids for option_id in result.ranked_option_ids):
            raise ValueError("evaluation returned an unknown option")
        return {
            "evaluation": result.model_copy(
                update={"request": request, "options": search.options, "credits": credits.credits}
            )
        }

    return node
