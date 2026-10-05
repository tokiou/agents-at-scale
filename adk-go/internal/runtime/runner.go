package runtime

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

const appName = "airline_rebooking"

// ErrNoPendingInput is returned when a resume is requested for a session that
// is not waiting for human input.
var ErrNoPendingInput = errors.New("session has no pending input")

// AgentRunner is the application boundary around the official ADK runner.
type AgentRunner struct {
	logger   *slog.Logger
	runner   *runner.Runner
	sessions session.Service
}

func NewAgentRunner(logger *slog.Logger, rootAgent adkagent.Agent, sessionService session.Service) (*AgentRunner, error) {
	adkRunner, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             rootAgent,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, err
	}
	return &AgentRunner{logger: logger, runner: adkRunner, sessions: sessionService}, nil
}

func (r *AgentRunner) Run(ctx context.Context, userID, sessionID, message string) iter.Seq2[*session.Event, error] {
	content := genai.NewContentFromText(message, genai.RoleUser)
	return r.run(ctx, userID, sessionID, content, len(message))
}

// Resume supplies a response to the pending workflow RequestInput of the
// session. The interrupt is looked up from the stored session events so
// callers only send the answer. The response is encoded as a FunctionResponse
// because that is the ADK protocol used to route HITL input back to the
// waiting node.
func (r *AgentRunner) Resume(ctx context.Context, userID, sessionID string, payload any) iter.Seq2[*session.Event, error] {
	interruptID, err := r.pendingInterrupt(ctx, userID, sessionID)
	if err != nil {
		return func(yield func(*session.Event, error) bool) { yield(nil, err) }
	}
	content := &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
			ID:   interruptID,
			Name: workflow.WorkflowInputFunctionCallName,
			Response: map[string]any{
				"payload": payload,
			},
		}}},
	}
	return r.run(ctx, userID, sessionID, content, 0)
}

func (r *AgentRunner) pendingInterrupt(ctx context.Context, userID, sessionID string) (string, error) {
	response, err := r.sessions.Get(ctx, &session.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoPendingInput, err)
	}
	interruptID := PendingInterrupt(response.Session.Events().All())
	if interruptID == "" {
		return "", ErrNoPendingInput
	}
	return interruptID, nil
}

// PendingInterrupt returns the latest RequestInput interrupt that has not
// been answered by a FunctionResponse, or "" when nothing is pending.
func PendingInterrupt(events iter.Seq[*session.Event]) string {
	var pending []string
	for event := range events {
		if event == nil {
			continue
		}
		if event.RequestedInput != nil {
			pending = append(pending, event.RequestedInput.InterruptID)
		}
		if event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part != nil && part.FunctionResponse != nil {
				pending = slices.DeleteFunc(pending, func(id string) bool { return id == part.FunctionResponse.ID })
			}
		}
	}
	if len(pending) == 0 {
		return ""
	}
	return pending[len(pending)-1]
}

func (r *AgentRunner) run(ctx context.Context, userID, sessionID string, content *genai.Content, messageLength int) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		r.logger.Info("agent run started", "user_id", userID, "session_id", sessionID, "message_length", messageLength)
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
