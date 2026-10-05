package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"log/slog"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	"github.com/tokiou/agents-at-scale/internal/runtime"
)

type fakeStore struct {
	statuses    []string
	current     map[string]string
	metadata    any
	timing      map[string]int64
	idempotency map[string]string
	locks       map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		current:     map[string]string{},
		timing:      map[string]int64{},
		idempotency: map[string]string{},
		locks:       map[string]string{},
	}
}

func (f *fakeStore) SetJobStatus(_ context.Context, jobID, status string) error {
	f.statuses = append(f.statuses, status)
	f.current[jobID] = status
	return nil
}

func (f *fakeStore) GetJobStatus(_ context.Context, jobID string) (string, error) {
	return f.current[jobID], nil
}

func (f *fakeStore) SetJobMetadata(_ context.Context, _ string, metadata any) error {
	f.metadata = metadata
	return nil
}

func (f *fakeStore) MarkJobTiming(_ context.Context, _ string, field string, at time.Time) error {
	if _, ok := f.timing[field]; !ok {
		f.timing[field] = at.UnixMilli()
	}
	return nil
}

func (f *fakeStore) SetJobTiming(_ context.Context, _ string, field string, value int64) error {
	f.timing[field] = value
	return nil
}

func (f *fakeStore) ClaimIdempotencyKey(_ context.Context, key, jobID string) (string, bool, error) {
	if owner, ok := f.idempotency[key]; ok {
		return owner, false, nil
	}
	f.idempotency[key] = jobID
	return jobID, true, nil
}

func (f *fakeStore) ReleaseIdempotencyKey(_ context.Context, key string) error {
	delete(f.idempotency, key)
	return nil
}

func (f *fakeStore) AcquireConversationLock(_ context.Context, conversation, token string, _ time.Duration) (bool, error) {
	if _, ok := f.locks[conversation]; ok {
		return false, nil
	}
	f.locks[conversation] = token
	return true, nil
}

func (f *fakeStore) ReleaseConversationLock(_ context.Context, conversation, token string) error {
	if f.locks[conversation] == token {
		delete(f.locks, conversation)
	}
	return nil
}

type fakeBroker struct {
	published []rabbitmq.Job
	retried   []rabbitmq.Job
	levels    []int
	dead      []rabbitmq.Job
}

func (b *fakeBroker) Publish(_ context.Context, job rabbitmq.Job) error {
	b.published = append(b.published, job)
	return nil
}

func (b *fakeBroker) Retry(_ context.Context, job rabbitmq.Job, level int) error {
	b.retried = append(b.retried, job)
	b.levels = append(b.levels, level)
	return nil
}

func (b *fakeBroker) DeadLetter(_ context.Context, job rabbitmq.Job) error {
	b.dead = append(b.dead, job)
	return nil
}

func (b *fakeBroker) Consume(int) (<-chan amqp091.Delivery, error) { return nil, nil }

type fakeAgentRunner struct {
	userID    string
	sessionID string
	message   string
	resume    any
	calls     int
	err       error
	event     *session.Event
}

func (r *fakeAgentRunner) Run(_ context.Context, userID, sessionID, message string) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		r.calls++
		r.userID, r.sessionID, r.message = userID, sessionID, message
		yield(r.event, r.err)
	}
}

func (r *fakeAgentRunner) Resume(_ context.Context, userID, sessionID string, payload any) iter.Seq2[*session.Event, error] {
	r.resume = payload
	return r.Run(context.Background(), userID, sessionID, "")
}

type fakeAcknowledger struct {
	acks     int
	nacks    int
	requeues int
}

func (a *fakeAcknowledger) Ack(_ uint64, _ bool) error {
	a.acks++
	return nil
}

func (a *fakeAcknowledger) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacks++
	if requeue {
		a.requeues++
	}
	return nil
}

func (a *fakeAcknowledger) Reject(_ uint64, _ bool) error { return nil }

type harness struct {
	store   *fakeStore
	broker  *fakeBroker
	runner  *fakeAgentRunner
	service *Service
}

func newHarness(runner *fakeAgentRunner) *harness {
	h := &harness{store: newFakeStore(), broker: &fakeBroker{}, runner: runner}
	h.service = New(slog.New(slog.NewTextHandler(io.Discard, nil)), h.store, h.broker, runner, Options{
		Concurrency: 1,
		MaxAttempts: 3,
		LockTTL:     time.Minute,
	})
	return h
}

func (h *harness) process(t *testing.T, job rabbitmq.Job) *fakeAcknowledger {
	t.Helper()
	body, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	acknowledger := &fakeAcknowledger{}
	h.service.process(context.Background(), amqp091.Delivery{Body: body, DeliveryTag: 1, Acknowledger: acknowledger})
	return acknowledger
}

const startPayload = `{"user_id":"user-1","session_id":"session-1","message":"change my flight"}`

func TestProcessRunsAgentBeforeAcknowledging(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	ack := h.process(t, rabbitmq.Job{ID: "job-1", Payload: json.RawMessage(startPayload)})

	if got, want := h.store.statuses, []string{StatusProcessing, StatusCompleted}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if ack.acks != 1 || ack.nacks != 0 {
		t.Fatalf("acks/nacks = %d/%d, want 1/0", ack.acks, ack.nacks)
	}
	if h.runner.userID != "user-1" || h.runner.sessionID != "session-1" || h.runner.message != "change my flight" {
		t.Fatalf("runner request = %q/%q/%q", h.runner.userID, h.runner.sessionID, h.runner.message)
	}
	if h.store.timing[TimingStarted] == 0 || h.store.timing[TimingFinished] == 0 || h.store.timing[TimingAttempts] != 1 {
		t.Fatalf("timing = %v", h.store.timing)
	}
	if len(h.store.locks) != 0 {
		t.Fatalf("conversation lock was not released: %v", h.store.locks)
	}
}

func TestProcessRetriesAgentFailureWithBackoff(t *testing.T) {
	h := newHarness(&fakeAgentRunner{err: errors.New("agent failed")})
	ack := h.process(t, rabbitmq.Job{ID: "job-2", Payload: json.RawMessage(startPayload)})

	if got, want := h.store.statuses, []string{StatusProcessing, StatusRetrying}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if len(h.broker.retried) != 1 || h.broker.retried[0].Attempt != 1 || h.broker.levels[0] != 1 {
		t.Fatalf("retried = %+v levels = %v", h.broker.retried, h.broker.levels)
	}
	if ack.acks != 1 || ack.nacks != 0 {
		t.Fatalf("acks/nacks = %d/%d, want 1/0", ack.acks, ack.nacks)
	}

	h.process(t, h.broker.retried[0])
	if len(h.broker.retried) != 2 || h.broker.levels[1] != 2 {
		t.Fatalf("second retry levels = %v", h.broker.levels)
	}
}

func TestProcessDeadLettersAfterMaxAttempts(t *testing.T) {
	h := newHarness(&fakeAgentRunner{err: errors.New("agent failed")})
	ack := h.process(t, rabbitmq.Job{ID: "job-3", Payload: json.RawMessage(startPayload), Attempt: 2})

	if got := h.store.current["job-3"]; got != StatusFailed {
		t.Fatalf("status = %q, want failed", got)
	}
	if len(h.broker.dead) != 1 || h.broker.dead[0].Error != "agent failed" || h.broker.dead[0].Attempt != 3 {
		t.Fatalf("dead = %+v", h.broker.dead)
	}
	if len(h.broker.retried) != 0 || ack.acks != 1 {
		t.Fatalf("retried = %d acks = %d", len(h.broker.retried), ack.acks)
	}
}

func TestProcessDeadLettersResumeWithoutPendingInput(t *testing.T) {
	h := newHarness(&fakeAgentRunner{err: runtime.ErrNoPendingInput})
	h.process(t, rabbitmq.Job{ID: "job-4", Payload: json.RawMessage(`{"user_id":"u","session_id":"s","resume":{"confirmed":false}}`)})

	if len(h.broker.dead) != 1 || len(h.broker.retried) != 0 || h.store.current["job-4"] != StatusFailed {
		t.Fatalf("dead = %d retried = %d status = %q", len(h.broker.dead), len(h.broker.retried), h.store.current["job-4"])
	}
}

func TestProcessDeadLettersInvalidPayloadWithoutRunningAgent(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	ack := h.process(t, rabbitmq.Job{ID: "job-5", Payload: json.RawMessage(`{"user_id":"user-1"}`)})

	if got, want := h.store.statuses, []string{StatusFailed}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if len(h.broker.dead) != 1 || ack.acks != 1 || h.runner.calls != 0 {
		t.Fatalf("dead = %d acks = %d calls = %d", len(h.broker.dead), ack.acks, h.runner.calls)
	}
}

func TestProcessSkipsDuplicateDeliveryOfFinishedJob(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	h.store.current["job-6"] = StatusCompleted
	ack := h.process(t, rabbitmq.Job{ID: "job-6", Payload: json.RawMessage(startPayload)})

	if h.runner.calls != 0 || ack.acks != 1 {
		t.Fatalf("calls = %d acks = %d", h.runner.calls, ack.acks)
	}
}

func TestProcessDelaysJobWhenConversationIsBusy(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	h.store.locks["user-1:session-1"] = "other-job"
	ack := h.process(t, rabbitmq.Job{ID: "job-7", Payload: json.RawMessage(startPayload)})

	if h.runner.calls != 0 || ack.acks != 1 {
		t.Fatalf("calls = %d acks = %d", h.runner.calls, ack.acks)
	}
	if len(h.broker.retried) != 1 || h.broker.retried[0].Attempt != 0 || h.broker.levels[0] != 1 {
		t.Fatalf("retried = %+v levels = %v", h.broker.retried, h.broker.levels)
	}
	if h.store.locks["user-1:session-1"] != "other-job" {
		t.Fatal("busy conversation lock was released by another job")
	}
}

func TestProcessMarksInterruptedRunAsWaitingForConfirmation(t *testing.T) {
	h := newHarness(&fakeAgentRunner{
		err: workflow.ErrNodeInterrupted,
		event: &session.Event{RequestedInput: &session.RequestInput{
			InterruptID: "confirm-1",
			Message:     "confirm",
			Payload:     map[string]any{"options": []any{}},
		}},
	})
	ack := h.process(t, rabbitmq.Job{ID: "job-8", Payload: json.RawMessage(startPayload)})

	if got, want := h.store.statuses, []string{StatusProcessing, StatusWaitingForConfirmation}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if ack.acks != 1 || ack.nacks != 0 {
		t.Fatalf("acks/nacks = %d/%d, want 1/0", ack.acks, ack.nacks)
	}
	metadata, ok := h.store.metadata.(WaitingMetadata)
	if !ok || metadata.UserID != "user-1" || metadata.SessionID != "session-1" || metadata.Message != "confirm" || metadata.Evaluation == nil {
		t.Fatalf("metadata = %#v", h.store.metadata)
	}
}

func TestProcessResumesWithConfirmationAnswer(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	h.process(t, rabbitmq.Job{ID: "job-9", Payload: json.RawMessage(`{"user_id":"user-1","session_id":"session-1","resume":{"confirmed":false}}`)})

	if got, want := h.store.statuses, []string{StatusProcessing, StatusCompleted}; !equalStrings(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	answer, ok := h.runner.resume.(map[string]any)
	if !ok || answer["confirmed"] != false {
		t.Fatalf("resume payload = %#v", h.runner.resume)
	}
}

func TestPublishIsIdempotentPerKey(t *testing.T) {
	h := newHarness(&fakeAgentRunner{})
	first, duplicate, err := h.service.Publish(context.Background(), json.RawMessage(startPayload), "key-1")
	if err != nil || duplicate {
		t.Fatalf("first publish = %q %v %v", first, duplicate, err)
	}
	second, duplicate, err := h.service.Publish(context.Background(), json.RawMessage(startPayload), "key-1")
	if err != nil || !duplicate || second != first {
		t.Fatalf("second publish = %q %v %v, want %q duplicate", second, duplicate, err, first)
	}
	if len(h.broker.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.broker.published))
	}
	if h.store.timing[TimingPublished] == 0 {
		t.Fatal("published timing was not recorded")
	}
}

func TestValidateAgentRequest(t *testing.T) {
	cases := map[string]struct {
		payload string
		valid   bool
	}{
		"message":             {`{"user_id":"u","session_id":"s","message":"hi"}`, true},
		"declined":            {`{"user_id":"u","session_id":"s","resume":{"confirmed":false}}`, true},
		"confirmed":           {`{"user_id":"u","session_id":"s","resume":{"confirmed":true,"selection":{}}}`, true},
		"missing session":     {`{"user_id":"u","message":"hi"}`, false},
		"message and resume":  {`{"user_id":"u","session_id":"s","message":"hi","resume":{"confirmed":false}}`, false},
		"neither":             {`{"user_id":"u","session_id":"s"}`, false},
		"missing confirmed":   {`{"user_id":"u","session_id":"s","resume":{}}`, false},
		"confirmed selection": {`{"user_id":"u","session_id":"s","resume":{"confirmed":true}}`, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var request AgentRequest
			if err := json.Unmarshal([]byte(tc.payload), &request); err != nil {
				t.Fatal(err)
			}
			if err := validateAgentRequest(request); (err == nil) != tc.valid {
				t.Fatalf("validateAgentRequest() error = %v, want valid %v", err, tc.valid)
			}
		})
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

type contextCapturingRunner struct {
	fakeAgentRunner
	ctx context.Context
}

func (r *contextCapturingRunner) Run(ctx context.Context, userID, sessionID, message string) iter.Seq2[*session.Event, error] {
	r.ctx = ctx
	return r.fakeAgentRunner.Run(ctx, userID, sessionID, message)
}

func TestEachJobRunsInItsOwnContextCancelledAfterward(t *testing.T) {
	runner := &contextCapturingRunner{}
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), newFakeStore(), &fakeBroker{}, runner, Options{Concurrency: 1, MaxAttempts: 3, LockTTL: time.Minute})
	body, err := json.Marshal(rabbitmq.Job{ID: "job-ctx", Payload: json.RawMessage(startPayload)})
	if err != nil {
		t.Fatal(err)
	}
	worker := context.Background()
	service.processWithJobContext(worker, amqp091.Delivery{Body: body, Acknowledger: &fakeAcknowledger{}})

	if runner.ctx == nil || runner.ctx.Err() == nil {
		t.Fatal("job context was not cancelled after the job finished")
	}
	if worker.Err() != nil {
		t.Fatal("worker context must stay alive")
	}
}
