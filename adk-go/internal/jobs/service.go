package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
)

type AgentRequest struct {
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	Message   string         `json:"message,omitempty"`
	Resume    *ResumeRequest `json:"resume,omitempty"`
}

type ResumeRequest struct {
	InterruptID string          `json:"interrupt_id"`
	Name        string          `json:"name"`
	Payload     json.RawMessage `json:"payload"`
}

type AgentRunner interface {
	Run(context.Context, string, string, string) iter.Seq2[*session.Event, error]
	Resume(context.Context, string, string, string, string, any) iter.Seq2[*session.Event, error]
}

type StatusStore interface {
	SetJobStatus(context.Context, string, string) error
}

type redisStatusStore struct{ client *redis.Client }

func NewRedisStatusStore(client *redis.Client) StatusStore {
	return redisStatusStore{client: client}
}

func (s redisStatusStore) SetJobStatus(ctx context.Context, jobID, status string) error {
	return redisplatform.SetJobStatus(ctx, s.client, jobID, status)
}

type Service struct {
	logger *slog.Logger
	status StatusStore
	rabbit *rabbitmq.Client
	runner AgentRunner
}

func New(logger *slog.Logger, status StatusStore, rabbitClient *rabbitmq.Client, runner AgentRunner) *Service {
	return &Service{logger: logger, status: status, rabbit: rabbitClient, runner: runner}
}

func (s *Service) Start(ctx context.Context) error {
	deliveries, err := s.rabbit.Consume()
	if err != nil {
		return err
	}
	go s.consume(ctx, deliveries)
	return nil
}

func (s *Service) Publish(ctx context.Context, payload json.RawMessage) (rabbitmq.Job, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	job := rabbitmq.Job{ID: uuid.NewString(), Payload: payload}
	if err := s.status.SetJobStatus(ctx, job.ID, "published"); err != nil {
		return rabbitmq.Job{}, err
	}
	if err := s.rabbit.Publish(ctx, job); err != nil {
		_ = s.status.SetJobStatus(ctx, job.ID, "failed")
		return rabbitmq.Job{}, err
	}
	return job, nil
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
			s.process(ctx, delivery)
		}
	}
}

func (s *Service) process(ctx context.Context, delivery amqp091.Delivery) {
	var job rabbitmq.Job
	if err := json.Unmarshal(delivery.Body, &job); err != nil {
		s.reject(delivery, "invalid job", err)
		return
	}
	var request AgentRequest
	if err := json.Unmarshal(job.Payload, &request); err != nil {
		s.fail(ctx, delivery, job.ID, "invalid job payload", err)
		return
	}
	if err := validateJob(job, request); err != nil {
		s.fail(ctx, delivery, job.ID, "invalid job payload", err)
		return
	}
	if s.runner == nil {
		s.fail(ctx, delivery, job.ID, "agent runner is missing", nil)
		return
	}
	if err := s.status.SetJobStatus(ctx, job.ID, "processing"); err != nil {
		s.fail(ctx, delivery, job.ID, "set processing status failed", err)
		return
	}
	s.logger.Info("job consumed", "job_id", job.ID, "user_id", request.UserID, "session_id", request.SessionID)
	var runErr error
	var waiting bool
	var events iter.Seq2[*session.Event, error]
	if request.Resume != nil {
		var payload any
		if len(request.Resume.Payload) > 0 {
			if err := json.Unmarshal(request.Resume.Payload, &payload); err != nil {
				s.fail(ctx, delivery, job.ID, "invalid resume payload", err)
				return
			}
		}
		events = s.runner.Resume(ctx, request.UserID, request.SessionID, request.Resume.InterruptID, request.Resume.Name, payload)
	} else {
		events = s.runner.Run(ctx, request.UserID, request.SessionID, request.Message)
	}
	for event, err := range events {
		if event != nil && len(event.LongRunningToolIDs) > 0 {
			waiting = true
		}
		if err != nil {
			if errors.Is(err, workflow.ErrNodeInterrupted) {
				waiting = true
				continue
			}
			runErr = err
		}
	}
	if runErr != nil {
		s.fail(ctx, delivery, job.ID, "agent run failed", runErr)
		return
	}
	status := "completed"
	if waiting {
		status = "waiting"
	}
	if err := s.status.SetJobStatus(ctx, job.ID, status); err != nil {
		s.fail(ctx, delivery, job.ID, "set completed status failed", err)
		return
	}
	if err := delivery.Ack(false); err != nil {
		s.logger.Error("ack job failed", "job_id", job.ID, "error", err)
	}
}

func validateJob(job rabbitmq.Job, request AgentRequest) error {
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("job id is required")
	}
	return validateAgentRequest(request)
}

func validateAgentRequest(request AgentRequest) error {
	if strings.TrimSpace(request.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(request.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	if request.Resume == nil && strings.TrimSpace(request.Message) == "" {
		return fmt.Errorf("message is required")
	}
	if request.Resume != nil {
		if strings.TrimSpace(request.Resume.InterruptID) == "" {
			return fmt.Errorf("resume interrupt_id is required")
		}
		if strings.TrimSpace(request.Resume.Name) == "" {
			return fmt.Errorf("resume name is required")
		}
		if strings.TrimSpace(request.Message) != "" {
			return fmt.Errorf("resume request cannot include message")
		}
		if err := validateResumePayload(request.Resume.Payload); err != nil {
			return err
		}
	}
	return nil
}

func validateResumePayload(payload json.RawMessage) error {
	if len(payload) == 0 || string(payload) == "null" {
		return fmt.Errorf("resume payload is required")
	}
	var response struct {
		Confirmed *bool           `json:"confirmed"`
		Selection json.RawMessage `json:"selection"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("resume payload must be a JSON object: %w", err)
	}
	if response.Confirmed == nil {
		return fmt.Errorf("resume payload confirmed is required")
	}
	if *response.Confirmed && len(response.Selection) == 0 {
		return fmt.Errorf("resume payload selection is required when confirmed")
	}
	return nil
}

func (s *Service) reject(delivery amqp091.Delivery, message string, err error) {
	s.logger.Error(message, "error", err)
	if nackErr := delivery.Nack(false, false); nackErr != nil {
		s.logger.Error("reject job failed", "error", nackErr)
	}
}

func (s *Service) fail(ctx context.Context, delivery amqp091.Delivery, jobID, message string, err error) {
	s.logger.Error(message, "job_id", jobID, "error", err)
	if strings.TrimSpace(jobID) != "" {
		if statusErr := s.status.SetJobStatus(ctx, jobID, "failed"); statusErr != nil {
			s.logger.Error("set failed status failed", "job_id", jobID, "error", statusErr)
		}
	}
	if nackErr := delivery.Nack(false, false); nackErr != nil {
		s.logger.Error("reject job failed", "job_id", jobID, "error", nackErr)
	}
}
