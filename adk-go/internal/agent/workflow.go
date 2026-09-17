package agent

import (
	"fmt"

	"github.com/tokiou/agents-at-scale/internal/airline"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/workflow"
)

func newWorkflow(llm model.LLM, airlineService *airline.Service) (adkagent.Agent, error) {
	understandRequest := newUnderstandRequestNode(llm)
	getReservation := newGetReservationNode(airlineService)
	searchAlternatives := newSearchAlternativesNode(airlineService)
	getTravelCredits := newGetTravelCreditsNode(airlineService)
	contextJoin := workflow.NewJoinNode("join_context")
	evaluateOptions := newEvaluateOptionsNode(llm)
	askConfirmation := newAskConfirmationNode()
	validateChange := newValidateChangeNode(airlineService)
	explainInvalidChange := newExplainInvalidChangeNode()
	executeRebooking := newExecuteRebookingNode(airlineService)
	verifyRebooking := newVerifyRebookingNode(airlineService)

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
