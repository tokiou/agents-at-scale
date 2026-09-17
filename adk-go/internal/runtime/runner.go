package runtime

import (
	"context"
	"iter"
	"log/slog"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
)

// AgentRunner is the application boundary around the official ADK runner.
type AgentRunner struct {
	logger *slog.Logger
	runner *runner.Runner
}

func NewAgentRunner(logger *slog.Logger, rootAgent adkagent.Agent, sessionService session.Service) (*AgentRunner, error) {
	adkRunner, err := runner.New(runner.Config{
		AppName:           "airline_rebooking",
		Agent:             rootAgent,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, err
	}
	return &AgentRunner{logger: logger, runner: adkRunner}, nil
}

func (r *AgentRunner) Run(ctx context.Context, userID, sessionID, message string) iter.Seq2[*session.Event, error] {
	content := genai.NewContentFromText(message, genai.RoleUser)
	return func(yield func(*session.Event, error) bool) {
		r.logger.Info("agent run started", "user_id", userID, "session_id", sessionID, "message_length", len(message))
		eventCount := 0
		for event, err := range r.runner.Run(ctx, userID, sessionID, content, adkagent.RunConfig{}) {
			if err != nil {
				r.logger.Error("agent run failed", "user_id", userID, "session_id", sessionID, "error", err)
			} else if event != nil {
				eventCount++
			}
			if !yield(event, err) {
				return
			}
		}
		r.logger.Info("agent run finished", "user_id", userID, "session_id", sessionID, "events", eventCount)
	}
}
