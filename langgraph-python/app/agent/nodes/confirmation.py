"""Human confirmation and deterministic selection validation nodes."""

from langgraph.types import interrupt
from pydantic import BaseModel

from app.agent.nodes.ports import ChangeValidator
from app.agent.state import (
    AgentState,
    ConfirmationResult,
    EvaluationResult,
    RebookingRequest,
    RebookingSelection,
    ValidationResult,
)


def _evaluation(value) -> EvaluationResult:
    data = dict(value.__dict__) if isinstance(value, EvaluationResult) else value
    if not isinstance(data, dict):
        raise ValueError("invalid evaluation")
    # Checkpoint serializers may restore nested Pydantic values as plain dicts.
    data = dict(data)
    request = data.pop("request", None)
    return EvaluationResult.model_construct(
        **data,
        request=RebookingRequest.model_validate(request) if isinstance(request, dict) else request,
    )


def _confirmation(value) -> ConfirmationResult:
    data = dict(value.__dict__) if isinstance(value, ConfirmationResult) else value
    if not isinstance(data, dict):
        raise ValueError("invalid confirmation")
    selection = data.get("selection")
    return ConfirmationResult.model_construct(
        confirmed=bool(data.get("confirmed", False)),
        selection=RebookingSelection.model_validate(selection) if selection else None,
        evaluation=_evaluation(data["evaluation"]),
    )


def _field(value, name: str):
    return value.get(name) if isinstance(value, dict) else getattr(value, name)


def _plain(value):
    if isinstance(value, BaseModel):
        return {key: _plain(item) for key, item in value.__dict__.items()}
    if isinstance(value, dict):
        return {key: _plain(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_plain(item) for item in value]
    return value


def ask_confirmation():
    def node(state: AgentState) -> dict:
        raw_evaluation = state.get("evaluation")
        if raw_evaluation is None:
            raise ValueError("confirmation requires an evaluation")
        evaluation = _evaluation(raw_evaluation)
        answer = interrupt({"message": "Select a rebooking option and confirm the change.", "evaluation": _plain(evaluation)})
        try:
            confirmation = _confirmation({**answer, "evaluation": evaluation})
        except (TypeError, ValueError) as exc:
            raise ValueError("invalid confirmation payload") from exc
        return {"confirmation": confirmation}

    return node


def validate_change(validator: ChangeValidator | None = None):
    async def node(state: AgentState) -> dict:
        raw_confirmation = state.get("confirmation")
        if raw_confirmation is None:
            raise ValueError("validation requires confirmation")
        confirmation = _confirmation(raw_confirmation)
        if not confirmation.confirmed:
            result = ValidationResult(
                evaluation=confirmation.evaluation,
                selection=confirmation.selection,
                reason="the rebooking was not confirmed",
                user_declined=True,
            )
            return {"validation": result, "route": "declined"}
        selection = confirmation.selection
        if selection is None:
            raise ValueError("confirmed selection is required")
        context = state.get("reservation_context")
        expected_segment_id = (
            confirmation.evaluation.request.segment_id
            or context.reservation.segments[0].segment.id
            if context is not None
            else confirmation.evaluation.request.segment_id
        )
        if expected_segment_id != selection.segment_id:
            return {
                "validation": ValidationResult(
                    evaluation=confirmation.evaluation,
                    selection=selection,
                    reason="the selected option belongs to another reservation segment",
                ),
                "route": "invalid",
            }
        offered = any(
            _field(_field(option, "flight"), "id") == selection.new_flight_id
            and _field(_field(option, "fare_class"), "id") == selection.new_fare_class_id
            for option in confirmation.evaluation.options
        )
        if not offered:
            result = ValidationResult(
                evaluation=confirmation.evaluation,
                selection=selection,
                reason="the selected option was not offered",
            )
            return {"validation": result, "route": "invalid"}
        try:
            domain_validation = await validator.validate_change(selection)
        except ValueError as exc:
            return {
                "validation": ValidationResult(
                    evaluation=confirmation.evaluation,
                    selection=selection,
                    reason=str(exc),
                ),
                "route": "invalid",
            }
        if domain_validation is False:
            return {
                "validation": ValidationResult(
                    evaluation=confirmation.evaluation,
                    selection=selection,
                    reason="the selected option failed domain validation",
                ),
                "route": "invalid",
            }
        result = ValidationResult(
            valid=True,
            evaluation=confirmation.evaluation,
            selection=selection,
            validation=domain_validation,
        )
        return {"validation": result, "route": "valid"}

    return node
