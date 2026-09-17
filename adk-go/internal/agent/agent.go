package agent

import (
	"github.com/tokiou/agents-at-scale/internal/airline"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
)

// New constructs the reusable root agent without a configured model.
//
// The workflow can be inspected or constructed with this function, but the
// understand_request node requires a model when it is executed.
func New() (adkagent.Agent, error) {
	return newWorkflow(nil, nil)
}

// NewWithModel constructs the reusable root agent with its LLM dependency.
func NewWithModel(llm model.LLM) (adkagent.Agent, error) {
	return newWorkflow(llm, nil)
}

// NewWithDependencies constructs the reusable root agent with its workflow dependencies.
func NewWithDependencies(llm model.LLM, airlineService *airline.Service) (adkagent.Agent, error) {
	return newWorkflow(llm, airlineService)
}
