package jobs

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
)

type Service struct {
	redis  *redis.Client
	rabbit *rabbitmq.Client
}

func New(redisClient *redis.Client, rabbitClient *rabbitmq.Client) *Service {
	return &Service{redis: redisClient, rabbit: rabbitClient}
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
	if err := redisplatform.SetJobStatus(ctx, s.redis, job.ID, "published"); err != nil {
		return rabbitmq.Job{}, err
	}
	if err := s.rabbit.Publish(ctx, job); err != nil {
		_ = redisplatform.SetJobStatus(ctx, s.redis, job.ID, "failed")
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
			var job rabbitmq.Job
			if err := json.Unmarshal(delivery.Body, &job); err != nil {
				slog.Default().Error("invalid job", "error", err)
				_ = delivery.Nack(false, false)
				continue
			}
			_ = redisplatform.SetJobStatus(ctx, s.redis, job.ID, "processing")
			slog.Default().Info("job consumed", "job_id", job.ID)
			_ = redisplatform.SetJobStatus(ctx, s.redis, job.ID, "completed")
			if err := delivery.Ack(false); err != nil {
				slog.Default().Error("ack job failed", "job_id", job.ID, "error", err)
			}
		}
	}
}
