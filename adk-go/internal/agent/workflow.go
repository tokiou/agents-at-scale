package agent

import (
	"fmt"
	"log/slog"

	"github.com/tokiou/agents-at-scale/internal/airline"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/workflow"
)

func newWorkflow(logger *slog.Logger, llm model.LLM, airlineService *airline.Service) (adkagent.Agent, error) {
	understandRequest := newUnderstandRequestNode(logger, llm)
	getReservation := newGetReservationNode(logger, airlineService)
	searchAlternatives := newSearchAlternativesNode(logger, airlineService)
	getTravelCredits := newGetTravelCreditsNode(logger, airlineService)
	contextJoin := workflow.NewJoinNode("join_context")
	evaluateOptions := newEvaluateOptionsNode(logger, llm)
	askConfirmation := newAskConfirmationNode(logger)
	validateChange := newValidateChangeNode(logger, airlineService)
	explainInvalidChange := newExplainInvalidChangeNode(logger)
	executeRebooking := newExecuteRebookingNode(logger, airlineService)
	verifyRebooking := newVerifyRebookingNode(logger, airlineService)

	builder := workflow.NewEdgeBuilder()
	builder.
		Add(workflow.Start, understandRequest).
		Add(understandRequest, getReservation).
		AddFanOut(getReservation, searchAlternatives, getTravelCredits).
		AddFanIn(contextJoin, searchAlternatives, getTravelCredits).
		Add(contextJoin, evaluateOptions).
		Add(evaluateOptions, askConfirmation).
		Add(askConfirmation, validateChange).
		AddRoutes(validateChange, map[string]workflow.Node{
			"invalid": explainInvalidChange,
			"valid":   executeRebooking,
		}).
		AddRoute(explainInvalidChange, evaluateOptions, workflow.StringRoute("retry")).
		Add(executeRebooking, verifyRebooking)

		// The invalid branch emits "retry" with the original evaluation inputs
		// so evaluate_options can be run again with fresh user-facing choices.
	root, err := workflowagent.New(workflowagent.Config{
		Name:        "airline_rebooking",
		Description: "Handles airline reservation rebooking workflows.",
		Edges:       builder.Build(),
	})
	if err != nil {
		return nil, fmt.Errorf("create airline rebooking workflow agent: %w", err)
	}
	return root, nil
}
