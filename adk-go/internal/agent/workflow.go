package agent

import (
	"fmt"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/workflow"
)

func newWorkflow() (adkagent.Agent, error) {
	understandRequest := newUnderstandRequestNode()
	loadReservation := newLoadReservationNode()
	searchAlternatives := newSearchAlternativesNode()
	loadTravelCredits := newLoadTravelCreditsNode()
	contextJoin := workflow.NewJoinNode("join_context")
	evaluateOptions := newEvaluateOptionsNode()
	askConfirmation := newAskConfirmationNode()
	validateChange := newValidateChangeNode()
	explainInvalidChange := newExplainInvalidChangeNode()
	executeRebooking := newExecuteRebookingNode()
	verifyRebooking := newVerifyRebookingNode()

	builder := workflow.NewEdgeBuilder()
	builder.
		Add(workflow.Start, understandRequest).
		Add(understandRequest, loadReservation).
		AddFanOut(loadReservation, searchAlternatives, loadTravelCredits).
		AddFanIn(contextJoin, searchAlternatives, loadTravelCredits).
		Add(contextJoin, evaluateOptions).
		Add(evaluateOptions, askConfirmation).
		Add(askConfirmation, validateChange).
		AddRoutes(validateChange, map[string]workflow.Node{
			"invalid": explainInvalidChange,
			"valid":   executeRebooking,
		}).
		Add(executeRebooking, verifyRebooking)

		// TODO: validateChange must become an emitting/routing node and emit
		// the route consumed by the conditional edges above. The placeholder
		// intentionally does not select either branch.
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
