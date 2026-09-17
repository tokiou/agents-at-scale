package agent

import (
	"log/slog"

	"github.com/tokiou/agents-at-scale/internal/airline"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
)

// New constructs the reusable root agent without a configured model.
//
// The workflow can be inspected or constructed with this function, but the
// understand_request node requires a model when it is executed.
func New(logger *slog.Logger) (adkagent.Agent, error) {
	return newWorkflow(logger, nil, nil)
}

// NewWithModel constructs the reusable root agent with its LLM dependency.
func NewWithModel(logger *slog.Logger, llm model.LLM) (adkagent.Agent, error) {
	return newWorkflow(logger, llm, nil)
}

// NewWithDependencies constructs the reusable root agent with its workflow dependencies.
func NewWithDependencies(logger *slog.Logger, llm model.LLM, airlineService *airline.Service) (adkagent.Agent, error) {
	return newWorkflow(logger, llm, airlineService)
}
