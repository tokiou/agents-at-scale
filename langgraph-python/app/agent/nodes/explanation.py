"""Invalid-selection explanation and retry bookkeeping."""

from app.agent.state import AgentState, FinalResult


def explain_invalid_change():
    def node(state: AgentState) -> dict:
        validation = state.get("validation")
        if validation is None:
            raise ValueError("invalid explanation requires validation")
        if validation.user_declined:
            return {
                "final": FinalResult(
                    status="declined",
                    message="The rebooking was not confirmed.",
                    selection=validation.selection,
                ),
                "route": "declined",
            }
        retry_count = state.get("retry_count", 0) + 1
        max_retries = state.get("max_retries", 2)
        if retry_count > max_retries:
            return {
                "final": FinalResult(
                    status="failed",
                    message=f"The selected rebooking is no longer valid: {validation.reason}",
                    selection=validation.selection,
                ),
                "retry_count": retry_count,
                "route": "invalid",
            }
        return {
            "error": validation.reason,
            "retry_count": retry_count,
            "route": "retry",
        }

    return node
