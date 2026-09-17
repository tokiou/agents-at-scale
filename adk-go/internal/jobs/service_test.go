package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"log/slog"
	"testing"

	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
)

type fakeStatusStore struct {
	statuses []string
}

func (s *fakeStatusStore) SetJobStatus(_ context.Context, _ string, status string) error {
	s.statuses = append(s.statuses, status)
	return nil
}

type fakeAgentRunner struct {
	userID    string
	sessionID string
	message   string
	err       error
	event     *session.Event
}

func (r *fakeAgentRunner) Run(_ context.Context, userID, sessionID, message string) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		r.userID, r.sessionID, r.message = userID, sessionID, message
		yield(r.event, r.err)
	}
}

func (r *fakeAgentRunner) Resume(_ context.Context, userID, sessionID, _, _ string, _ any) iter.Seq2[*session.Event, error] {
	return r.Run(context.Background(), userID, sessionID, "")
}

type fakeAcknowledger struct {
	acks  int
	nacks int
}

func (a *fakeAcknowledger) Ack(_ uint64, _ bool) error {
	a.acks++
	return nil
}

func (a *fakeAcknowledger) Nack(_ uint64, _ bool, _ bool) error {
	a.nacks++
	return nil
}

func (a *fakeAcknowledger) Reject(_ uint64, _ bool) error { return nil }

func newTestService(status *fakeStatusStore, runner AgentRunner) *Service {
	return &Service{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		status: status,
		runner: runner,
	}
}

func makeDelivery(t *testing.T, job rabbitmq.Job, acknowledger *fakeAcknowledger) amqp091.Delivery {
	t.Helper()
	body, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	return amqp091.Delivery{Body: body, DeliveryTag: 1, Acknowledger: acknowledger}
}

func TestProcessRunsAgentBeforeAcknowledging(t *testing.T) {
	status := &fakeStatusStore{}
	runner := &fakeAgentRunner{}
	acknowledger := &fakeAcknowledger{}
	service := newTestService(status, runner)
	service.process(context.Background(), makeDelivery(t, rabbitmq.Job{
		ID:      "job-1",
		Payload: json.RawMessage(`{"user_id":"user-1","session_id":"session-1","message":"change my flight"}`),
	}, acknowledger))

	if got, want := len(status.statuses), 2; got != want {
		t.Fatalf("status writes = %d, want %d", got, want)
	}
	if status.statuses[0] != "processing" || status.statuses[1] != "completed" {
		t.Fatalf("statuses = %v, want processing then completed", status.statuses)
	}
	if acknowledger.acks != 1 || acknowledger.nacks != 0 {
		t.Fatalf("acks/nacks = %d/%d, want 1/0", acknowledger.acks, acknowledger.nacks)
	}
	if runner.userID != "user-1" || runner.sessionID != "session-1" || runner.message != "change my flight" {
		t.Fatalf("runner request = %q/%q/%q", runner.userID, runner.sessionID, runner.message)
	}
}

func TestProcessRejectsAgentFailure(t *testing.T) {
	status := &fakeStatusStore{}
	runner := &fakeAgentRunner{err: errors.New("agent failed")}
	acknowledger := &fakeAcknowledger{}
	service := newTestService(status, runner)
	service.process(context.Background(), makeDelivery(t, rabbitmq.Job{
		ID:      "job-2",
		Payload: json.RawMessage(`{"user_id":"user-1","session_id":"session-1","message":"change my flight"}`),
	}, acknowledger))

	if got, want := status.statuses, []string{"processing", "failed"}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if acknowledger.acks != 0 || acknowledger.nacks != 1 {
		t.Fatalf("acks/nacks = %d/%d, want 0/1", acknowledger.acks, acknowledger.nacks)
	}
}

func TestProcessRejectsInvalidPayloadWithoutRunningAgent(t *testing.T) {
	status := &fakeStatusStore{}
	runner := &fakeAgentRunner{}
	acknowledger := &fakeAcknowledger{}
	service := newTestService(status, runner)
	service.process(context.Background(), makeDelivery(t, rabbitmq.Job{
		ID:      "job-3",
		Payload: json.RawMessage(`{"user_id":"user-1"}`),
	}, acknowledger))

	if got, want := status.statuses, []string{"failed"}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if acknowledger.acks != 0 || acknowledger.nacks != 1 {
		t.Fatalf("acks/nacks = %d/%d, want 0/1", acknowledger.acks, acknowledger.nacks)
	}
	if runner.userID != "" {
		t.Fatal("agent runner was called for invalid payload")
	}
}

func TestProcessMarksInterruptedRunAsWaiting(t *testing.T) {
	status := &fakeStatusStore{}
	runner := &fakeAgentRunner{
		err:   workflow.ErrNodeInterrupted,
		event: &session.Event{LongRunningToolIDs: []string{"confirm-1"}},
	}
	acknowledger := &fakeAcknowledger{}
	service := newTestService(status, runner)
	service.process(context.Background(), makeDelivery(t, rabbitmq.Job{
		ID:      "job-4",
		Payload: json.RawMessage(`{"user_id":"user-1","session_id":"session-1","message":"change my flight"}`),
	}, acknowledger))

	if got, want := status.statuses, []string{"processing", "waiting"}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if acknowledger.acks != 1 || acknowledger.nacks != 0 {
		t.Fatalf("acks/nacks = %d/%d, want 1/0", acknowledger.acks, acknowledger.nacks)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
