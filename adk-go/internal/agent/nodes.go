package agent

import (
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/workflow"
)

func newUnderstandRequestNode() workflow.Node {
	return workflow.NewFunctionNode(
		"understand_request",
		func(ctx adkagent.Context, input string) (RebookingRequest, error) {
			/*
				TODO: Use an LLM to understand the user's request and extract the
				booking reference, requested changes, constraints, and preferences.
				Do not parse or infer request data in this skeleton.
			*/
			return RebookingRequest{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newLoadReservationNode() workflow.Node {
	return workflow.NewFunctionNode(
		"load_reservation",
		func(ctx adkagent.Context, input RebookingRequest) (ReservationContext, error) {
			/*
				TODO: Load the reservation and relevant segment through AirlineService,
				verify that it exists, and map expected domain errors. Do not call a
				service, repository, or database in this skeleton.
			*/
			return ReservationContext{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newSearchAlternativesNode() workflow.Node {
	return workflow.NewFunctionNode(
		"search_alternatives",
		func(ctx adkagent.Context, input ReservationContext) (SearchAlternativesResult, error) {
			/*
				TODO: Search deterministic flight and fare alternatives for the
				reservation's route while honoring the request and airline rules.
				Do not access availability, fares, or a database in this skeleton.
			*/
			return SearchAlternativesResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newLoadTravelCreditsNode() workflow.Node {
	return workflow.NewFunctionNode(
		"load_travel_credits",
		func(ctx adkagent.Context, input ReservationContext) (TravelCreditsResult, error) {
			/*
				TODO: Identify the reservation owner and load non-expired travel
				credits, preserving their currency information. Do not query or
				calculate credits in this skeleton.
			*/
			return TravelCreditsResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newEvaluateOptionsNode() workflow.Node {
	return workflow.NewFunctionNode(
		"evaluate_options",
		func(ctx adkagent.Context, input map[string]any) (EvaluationResult, error) {
			/*
				TODO: Combine the native JoinNode outputs, apply the user's
				preferences, and rank or summarize useful alternatives. This will
				likely become an LLM-backed step; do not evaluate data here.
			*/
			return EvaluationResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newAskConfirmationNode() workflow.Node {
	return workflow.NewFunctionNode(
		"ask_confirmation",
		func(ctx adkagent.Context, input EvaluationResult) (ConfirmationResult, error) {
			/*
				TODO: Present options, request the user's selection, pause through
				ADK's HITL request-input primitives, and convert the response into a
				structured confirmation. Do not pause or resume in this skeleton.
			*/
			return ConfirmationResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newValidateChangeNode() workflow.Node {
	return workflow.NewFunctionNode(
		"validate_change",
		func(ctx adkagent.Context, input ConfirmationResult) (ValidationResult, error) {
			/*
				TODO: Deterministically revalidate the reservation, selected flight
				and fare, inventory, change rules, and travel credit immediately
				before mutation. Later this node must emit the valid/invalid route;
				do not fake validity or route events here.
			*/
			return ValidationResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newExplainInvalidChangeNode() workflow.Node {
	return workflow.NewFunctionNode(
		"explain_invalid_change",
		func(ctx adkagent.Context, input ValidationResult) (FinalResult, error) {
			/*
				TODO: Produce a user-facing explanation for a failed validation and,
				when appropriate, suggest selecting another option. Do not generate
				or fabricate a response in this skeleton.
			*/
			return FinalResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newExecuteRebookingNode() workflow.Node {
	return workflow.NewFunctionNode(
		"execute_rebooking",
		func(ctx adkagent.Context, input ValidationResult) (RebookingExecutionResult, error) {
			/*
				TODO: Atomically lock and revalidate state, update the reservation,
				adjust inventory, consume applicable credit, record the change, and
				commit through the airline service. Do not mutate anything here.
			*/
			return RebookingExecutionResult{}, nil
		},
		workflow.NodeConfig{},
	)
}

func newVerifyRebookingNode() workflow.Node {
	return workflow.NewFunctionNode(
		"verify_rebooking",
		func(ctx adkagent.Context, input RebookingExecutionResult) (FinalResult, error) {
			/*
				TODO: Reload the reservation, verify the expected persisted flight
				and fare, and produce the final workflow result. Do not access a
				service or database in this skeleton.
			*/
			return FinalResult{}, nil
		},
		workflow.NodeConfig{},
	)
}
