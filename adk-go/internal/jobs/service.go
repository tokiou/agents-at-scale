package jobs

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

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

func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(s.publish)
}

type jobRequest struct {
	Payload json.RawMessage `json:"payload"`
}

func (s *Service) publish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request jobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	job := rabbitmq.Job{ID: uuid.NewString(), Payload: request.Payload}
	if err := redisplatform.SetJobStatus(r.Context(), s.redis, job.ID, "published"); err != nil {
		http.Error(w, "save job status failed", http.StatusInternalServerError)
		return
	}
	if err := s.rabbit.Publish(r.Context(), job); err != nil {
		_ = redisplatform.SetJobStatus(r.Context(), s.redis, job.ID, "failed")
		http.Error(w, "publish job failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": job.ID, "status": "published"})
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
				log.Printf("invalid job: %v", err)
				_ = delivery.Nack(false, false)
				continue
			}
			_ = redisplatform.SetJobStatus(ctx, s.redis, job.ID, "processing")
			log.Printf("job consumed id=%s payload=%s", job.ID, job.Payload)
			_ = redisplatform.SetJobStatus(ctx, s.redis, job.ID, "completed")
			if err := delivery.Ack(false); err != nil {
				log.Printf("ack job id=%s: %v", job.ID, err)
			}
		}
	}
}
