package agent

import (
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
)

// New constructs the reusable root agent without a configured model.
//
// The workflow can be inspected or constructed with this function, but the
// understand_request node requires a model when it is executed.
func New() (adkagent.Agent, error) {
	return newWorkflow(nil)
}

// NewWithModel constructs the reusable root agent with its LLM dependency.
func NewWithModel(llm model.LLM) (adkagent.Agent, error) {
	return newWorkflow(llm)
}
