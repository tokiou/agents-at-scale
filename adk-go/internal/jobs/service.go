package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
	"github.com/tokiou/agents-at-scale/internal/runtime"
)

// Job lifecycle statuses stored in Redis. Both runtimes use the same values.
const (
	StatusPublished              = "published"
	StatusProcessing             = "processing"
	StatusWaitingForConfirmation = "waiting_for_confirmation"
	StatusCompleted              = "completed"
	StatusFailed                 = "failed"
	StatusRetrying               = "retrying"
)

// Timing fields stored in job:<id>:timing as epoch milliseconds.
const (
	TimingPublished = "published_ms"
	TimingStarted   = "started_ms"
	TimingFinished  = "finished_ms"
	TimingAttempts  = "attempts"
)

// AgentRequest is the queued chat payload. Exactly one of Message or Resume
// is set; Resume carries the answer to a pending confirmation.
type AgentRequest struct {
	UserID    string          `json:"user_id"`
	SessionID string          `json:"session_id"`
	Message   string          `json:"message,omitempty"`
	Resume    json.RawMessage `json:"resume,omitempty"`
}

// WaitingMetadata is stored when a job pauses for confirmation so a client
// can build the resume request.
type WaitingMetadata struct {
	UserID     string `json:"user_id"`
	SessionID  string `json:"session_id"`
	Message    string `json:"message"`
	Evaluation any    `json:"evaluation"`
}

type AgentRunner interface {
	Run(context.Context, string, string, string) iter.Seq2[*session.Event, error]
	Resume(context.Context, string, string, any) iter.Seq2[*session.Event, error]
}

// Store keeps job state and coordination data in Redis.
type Store interface {
	SetJobStatus(context.Context, string, string) error
	GetJobStatus(context.Context, string) (string, error)
	SetJobMetadata(context.Context, string, any) error
	MarkJobTiming(context.Context, string, string, time.Time) error
	SetJobTiming(context.Context, string, string, int64) error
	ClaimIdempotencyKey(context.Context, string, string) (string, bool, error)
	ReleaseIdempotencyKey(context.Context, string) error
	AcquireConversationLock(context.Context, string, string, time.Duration) (bool, error)
	ReleaseConversationLock(context.Context, string, string) error
}

// Broker is the durable job transport.
type Broker interface {
	Publish(context.Context, rabbitmq.Job) error
	Retry(context.Context, rabbitmq.Job, int) error
	DeadLetter(context.Context, rabbitmq.Job) error
	Consume(int) (<-chan amqp091.Delivery, error)
}

type redisStore struct{ client *redis.Client }

func NewRedisStore(client *redis.Client) Store {
	return redisStore{client: client}
}

func (s redisStore) SetJobStatus(ctx context.Context, jobID, status string) error {
	return redisplatform.SetJobStatus(ctx, s.client, jobID, status)
}

func (s redisStore) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	return redisplatform.GetJobStatus(ctx, s.client, jobID)
}

func (s redisStore) SetJobMetadata(ctx context.Context, jobID string, metadata any) error {
	return redisplatform.SetJobMetadata(ctx, s.client, jobID, metadata)
}

func (s redisStore) MarkJobTiming(ctx context.Context, jobID, field string, at time.Time) error {
	return redisplatform.MarkJobTiming(ctx, s.client, jobID, field, at)
}

func (s redisStore) SetJobTiming(ctx context.Context, jobID, field string, value int64) error {
	return redisplatform.SetJobTiming(ctx, s.client, jobID, field, value)
}

func (s redisStore) ClaimIdempotencyKey(ctx context.Context, key, jobID string) (string, bool, error) {
	return redisplatform.ClaimIdempotencyKey(ctx, s.client, key, jobID)
}

func (s redisStore) ReleaseIdempotencyKey(ctx context.Context, key string) error {
	return redisplatform.ReleaseIdempotencyKey(ctx, s.client, key)
}

func (s redisStore) AcquireConversationLock(ctx context.Context, conversation, token string, ttl time.Duration) (bool, error) {
	return redisplatform.AcquireConversationLock(ctx, s.client, conversation, token, ttl)
}

func (s redisStore) ReleaseConversationLock(ctx context.Context, conversation, token string) error {
	return redisplatform.ReleaseConversationLock(ctx, s.client, conversation, token)
}

// Options tunes the worker.
type Options struct {
	// Concurrency is the number of jobs run at once; zero disables consuming.
	Concurrency int
	// MaxAttempts is the total number of processing attempts per job.
	MaxAttempts int
	// LockTTL bounds how long a conversation lock is held if a worker dies.
	LockTTL time.Duration
}

type Service struct {
	logger  *slog.Logger
	store   Store
	broker  Broker
	runner  AgentRunner
	options Options
	now     func() time.Time
}

func New(logger *slog.Logger, store Store, broker Broker, runner AgentRunner, options Options) *Service {
	if options.MaxAttempts < 1 {
		options.MaxAttempts = 1
	}
	return &Service{logger: logger, store: store, broker: broker, runner: runner, options: options, now: time.Now}
}

func (s *Service) Start(ctx context.Context) error {
	if s.options.Concurrency <= 0 {
		s.logger.Info("job consumer disabled")
		return nil
	}
	deliveries, err := s.broker.Consume(s.options.Concurrency)
	if err != nil {
		return err
	}
	for range s.options.Concurrency {
		go s.consume(ctx, deliveries)
	}
	s.logger.Info("job consumer started", "concurrency", s.options.Concurrency, "max_attempts", s.options.MaxAttempts)
	return nil
}

// Publish queues a chat payload. When idempotencyKey is set and was already
// used, the existing job ID is returned with duplicate=true and nothing is
// queued.
func (s *Service) Publish(ctx context.Context, payload json.RawMessage, idempotencyKey string) (string, bool, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	job := rabbitmq.Job{ID: uuid.NewString(), Payload: payload}
	if idempotencyKey != "" {
		owner, claimed, err := s.store.ClaimIdempotencyKey(ctx, idempotencyKey, job.ID)
		if err != nil {
			return "", false, err
		}
		if !claimed {
			return owner, true, nil
		}
	}
	if err := s.publishNew(ctx, job); err != nil {
		if idempotencyKey != "" {
			_ = s.store.ReleaseIdempotencyKey(ctx, idempotencyKey)
		}
		return "", false, err
	}
	return job.ID, false, nil
}

func (s *Service) publishNew(ctx context.Context, job rabbitmq.Job) error {
	if err := s.store.SetJobStatus(ctx, job.ID, StatusPublished); err != nil {
		return err
	}
	if err := s.store.MarkJobTiming(ctx, job.ID, TimingPublished, s.now()); err != nil {
		return err
	}
	if err := s.broker.Publish(ctx, job); err != nil {
		_ = s.store.SetJobStatus(ctx, job.ID, StatusFailed)
		return err
	}
	return nil
}

func (s *Service) consume(ctx context.Context, deliveries <-chan amqp091.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			s.processWithJobContext(ctx, delivery)
		}
	}
}

// processWithJobContext gives every job its own cancellable context. ADK
// v2.0.0 derives a cancellable context per workflow node and drops it without
// cancelling, so derived contexts (holding the invocation and its session
// events) stay registered on the parent; cancelling the job context when the
// job ends releases them instead of accumulating them on the worker context.
func (s *Service) processWithJobContext(ctx context.Context, delivery amqp091.Delivery) {
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.process(jobCtx, delivery)
}

func isTerminal(status string) bool {
	return status == StatusCompleted || status == StatusWaitingForConfirmation || status == StatusFailed
}

func (s *Service) process(ctx context.Context, delivery amqp091.Delivery) {
	var job rabbitmq.Job
	if err := json.Unmarshal(delivery.Body, &job); err != nil || strings.TrimSpace(job.ID) == "" {
		s.reject(delivery, "invalid job envelope", err)
		return
	}
	// RabbitMQ delivers at least once; a job that already reached a final
	// state is a duplicate delivery and must not run the agent again.
	status, err := s.store.GetJobStatus(ctx, job.ID)
	if err != nil {
		s.retry(ctx, delivery, job, "get job status failed", err)
		return
	}
	if isTerminal(status) {
		s.logger.Warn("duplicate job delivery skipped", "job_id", job.ID, "status", status)
		s.ack(delivery, job.ID)
		return
	}

	var request AgentRequest
	if err := json.Unmarshal(job.Payload, &request); err != nil {
		s.deadLetter(ctx, delivery, job, "invalid job payload", err)
		return
	}
	if err := validateAgentRequest(request); err != nil {
		s.deadLetter(ctx, delivery, job, "invalid job payload", err)
		return
	}
	if s.runner == nil {
		s.deadLetter(ctx, delivery, job, "agent runner is missing", errors.New("agent runner is required"))
		return
	}

	// One job per conversation at a time: a second job for the same session
	// waits in the first retry queue without consuming an attempt.
	conversation := request.UserID + ":" + request.SessionID
	locked, err := s.store.AcquireConversationLock(ctx, conversation, job.ID, s.options.LockTTL)
	if err != nil {
		s.retry(ctx, delivery, job, "acquire conversation lock failed", err)
		return
	}
	if !locked {
		s.logger.Info("conversation busy, delaying job", "job_id", job.ID, "conversation", conversation)
		s.delay(ctx, delivery, job, 1)
		return
	}
	defer func() {
		if err := s.store.ReleaseConversationLock(context.WithoutCancel(ctx), conversation, job.ID); err != nil {
			s.logger.Error("release conversation lock failed", "job_id", job.ID, "error", err)
		}
	}()

	if err := s.markStarted(ctx, job); err != nil {
		s.retry(ctx, delivery, job, "set processing status failed", err)
		return
	}
	s.logger.Info("job consumed", "job_id", job.ID, "user_id", request.UserID, "session_id", request.SessionID, "attempt", job.Attempt+1)

	requested, err := s.run(ctx, request)
	if err != nil {
		if errors.Is(err, runtime.ErrNoPendingInput) {
			s.deadLetter(ctx, delivery, job, "agent run failed", err)
			return
		}
		s.retry(ctx, delivery, job, "agent run failed", err)
		return
	}
	if err := s.finish(ctx, job, request, requested); err != nil {
		s.retry(ctx, delivery, job, "set final status failed", err)
		return
	}
	s.ack(delivery, job.ID)
}

func (s *Service) markStarted(ctx context.Context, job rabbitmq.Job) error {
	if err := s.store.SetJobStatus(ctx, job.ID, StatusProcessing); err != nil {
		return err
	}
	if err := s.store.MarkJobTiming(ctx, job.ID, TimingStarted, s.now()); err != nil {
		return err
	}
	return s.store.SetJobTiming(ctx, job.ID, TimingAttempts, int64(job.Attempt+1))
}

func (s *Service) run(ctx context.Context, request AgentRequest) (*session.RequestInput, error) {
	var events iter.Seq2[*session.Event, error]
	if len(request.Resume) > 0 {
		var payload any
		if err := json.Unmarshal(request.Resume, &payload); err != nil {
			return nil, err
		}
		events = s.runner.Resume(ctx, request.UserID, request.SessionID, payload)
	} else {
		events = s.runner.Run(ctx, request.UserID, request.SessionID, request.Message)
	}
	var runErr error
	var requested *session.RequestInput
	for event, err := range events {
		if event != nil && event.RequestedInput != nil {
			requested = event.RequestedInput
		}
		if err != nil && !errors.Is(err, workflow.ErrNodeInterrupted) {
			runErr = err
		}
	}
	return requested, runErr
}

func (s *Service) finish(ctx context.Context, job rabbitmq.Job, request AgentRequest, requested *session.RequestInput) error {
	status := StatusCompleted
	if requested != nil {
		status = StatusWaitingForConfirmation
		s.logger.Info("job waiting for confirmation", "job_id", job.ID, "interrupt_id", requested.InterruptID)
		if err := s.store.SetJobMetadata(ctx, job.ID, WaitingMetadata{
			UserID:     request.UserID,
			SessionID:  request.SessionID,
			Message:    requested.Message,
			Evaluation: requested.Payload,
		}); err != nil {
			return err
		}
	}
	if err := s.store.SetJobTiming(ctx, job.ID, TimingFinished, s.now().UnixMilli()); err != nil {
		return err
	}
	return s.store.SetJobStatus(ctx, job.ID, status)
}

// retry schedules another attempt with exponential backoff, or dead-letters
// the job when its attempts are exhausted.
func (s *Service) retry(ctx context.Context, delivery amqp091.Delivery, job rabbitmq.Job, message string, err error) {
	job.Attempt++
	if job.Attempt >= s.options.MaxAttempts {
		s.deadLetter(ctx, delivery, job, message, err)
		return
	}
	s.logger.Warn(message+", retrying", "job_id", job.ID, "attempt", job.Attempt, "error", err)
	if statusErr := s.store.SetJobStatus(ctx, job.ID, StatusRetrying); statusErr != nil {
		s.logger.Error("set retrying status failed", "job_id", job.ID, "error", statusErr)
	}
	s.delay(ctx, delivery, job, job.Attempt)
}

func (s *Service) delay(ctx context.Context, delivery amqp091.Delivery, job rabbitmq.Job, level int) {
	if err := s.broker.Retry(ctx, job, level); err != nil {
		s.logger.Error("schedule retry failed, requeueing", "job_id", job.ID, "error", err)
		s.nack(delivery, job.ID, true)
		return
	}
	s.ack(delivery, job.ID)
}

func (s *Service) deadLetter(ctx context.Context, delivery amqp091.Delivery, job rabbitmq.Job, message string, err error) {
	s.logger.Error(message, "job_id", job.ID, "attempt", job.Attempt, "error", err)
	if err != nil {
		job.Error = err.Error()
	}
	if statusErr := s.store.SetJobStatus(ctx, job.ID, StatusFailed); statusErr != nil {
		s.logger.Error("set failed status failed", "job_id", job.ID, "error", statusErr)
	}
	if timingErr := s.store.SetJobTiming(ctx, job.ID, TimingFinished, s.now().UnixMilli()); timingErr != nil {
		s.logger.Error("set finished timing failed", "job_id", job.ID, "error", timingErr)
	}
	if publishErr := s.broker.DeadLetter(ctx, job); publishErr != nil {
		s.logger.Error("dead-letter job failed", "job_id", job.ID, "error", publishErr)
		s.nack(delivery, job.ID, false)
		return
	}
	s.ack(delivery, job.ID)
}

func (s *Service) ack(delivery amqp091.Delivery, jobID string) {
	if err := delivery.Ack(false); err != nil {
		s.logger.Error("ack job failed", "job_id", jobID, "error", err)
	}
}

func (s *Service) nack(delivery amqp091.Delivery, jobID string, requeue bool) {
	if err := delivery.Nack(false, requeue); err != nil {
		s.logger.Error("reject job failed", "job_id", jobID, "error", err)
	}
}

func validateAgentRequest(request AgentRequest) error {
	if strings.TrimSpace(request.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(request.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	hasMessage := strings.TrimSpace(request.Message) != ""
	hasResume := len(request.Resume) > 0 && string(request.Resume) != "null"
	if hasMessage == hasResume {
		return fmt.Errorf("exactly one of message or resume is required")
	}
	if hasResume {
		return ValidateResume(request.Resume)
	}
	return nil
}

// ValidateResume checks the confirmation answer shared by both runtimes:
// {"confirmed": bool, "selection": {...}}, with selection required when
// confirmed.
func ValidateResume(payload json.RawMessage) error {
	var response struct {
		Confirmed *bool           `json:"confirmed"`
		Selection json.RawMessage `json:"selection"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("resume must be a JSON object: %w", err)
	}
	if response.Confirmed == nil {
		return fmt.Errorf("resume.confirmed is required")
	}
	if *response.Confirmed && (len(response.Selection) == 0 || string(response.Selection) == "null") {
		return fmt.Errorf("resume.selection is required when confirmed")
	}
	return nil
}

func (s *Service) reject(delivery amqp091.Delivery, message string, err error) {
	s.logger.Error(message, "error", err)
	if nackErr := delivery.Nack(false, false); nackErr != nil {
		s.logger.Error("reject job failed", "error", nackErr)
	}
}
