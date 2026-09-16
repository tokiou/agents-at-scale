package agent

import adkagent "google.golang.org/adk/v2/agent"

// New constructs the reusable root agent for the airline rebooking workflow.
func New() (adkagent.Agent, error) {
	return newWorkflow()
}
