"""Rebooking execution and post-commit verification nodes."""

from app.agent.state import AgentState, FinalResult
from app.airline.schemas import RebookingSelectionSchema


def execute_rebooking(airline):
    async def node(state: AgentState) -> dict:
        validation = state.get("validation")
        if validation is None or not validation.valid or validation.selection is None:
            raise ValueError("execution requires a valid selection")
        try:
            result = await airline.execute_rebooking(
                RebookingSelectionSchema.model_validate(validation.selection.model_dump())
            )
        except Exception as exc:
            return {
                "final": FinalResult(
                    status="failed",
                    message="The rebooking could not be executed.",
                    selection=validation.selection,
                    validation=validation.validation,
                ),
                "error": str(exc),
                "route": "failed",
            }
        return {"execution": result, "route": "completed"}

    return node


def verify_rebooking(airline):
    async def node(state: AgentState) -> dict:
        validation = state.get("validation")
        execution = state.get("execution")
        if validation is None or validation.selection is None or execution is None:
            raise ValueError("verification requires execution state")
        selection = RebookingSelectionSchema.model_validate(validation.selection.model_dump())
        try:
            result = await airline.verify_rebooking(selection)
        except Exception as exc:
            return {
                "final": FinalResult(
                    status="failed",
                    message="The rebooking could not be verified.",
                    selection=validation.selection,
                    validation=validation.validation,
                    execution=execution,
                ),
                "error": str(exc),
                "route": "failed",
            }
        return {
            "verification": result,
            "final": FinalResult(
                status="completed",
                message="The rebooking was completed successfully.",
                selection=validation.selection,
                validation=validation.validation,
                execution=execution,
                verification=result,
            ),
            "route": "completed",
        }

    return node
