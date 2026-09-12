package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
	"github.com/tokiou/agents-at-scale/internal/config"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres"
	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
)

func Run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
	if err != nil {
		return err
	}
	defer db.Close()
	redisClient, err := redisplatform.New(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	rabbitClient, err := rabbitmq.New(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		return err
	}
	defer rabbitClient.Close()

	app := &application{redis: redisClient, rabbit: rabbitClient}
	deliveries, err := rabbitClient.Consume()
	if err != nil {
		return err
	}
	go app.consumeJobs(ctx, deliveries)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.health)
	mux.HandleFunc("/jobs", app.publishJob)
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: mux,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

type application struct {
	redis  *redis.Client
	rabbit *rabbitmq.Client
}

type jobRequest struct {
	Payload json.RawMessage `json:"payload"`
}

func (a *application) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (a *application) publishJob(w http.ResponseWriter, r *http.Request) {
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
	if err := redisplatform.SetJobStatus(r.Context(), a.redis, job.ID, "published"); err != nil {
		http.Error(w, "save job status failed", http.StatusInternalServerError)
		return
	}
	if err := a.rabbit.Publish(r.Context(), job); err != nil {
		_ = redisplatform.SetJobStatus(r.Context(), a.redis, job.ID, "failed")
		http.Error(w, "publish job failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": job.ID, "status": "published"})
}

func (a *application) consumeJobs(ctx context.Context, deliveries <-chan amqp091.Delivery) {
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
			_ = redisplatform.SetJobStatus(ctx, a.redis, job.ID, "processing")
			log.Printf("job consumed id=%s payload=%s", job.ID, job.Payload)
			_ = redisplatform.SetJobStatus(ctx, a.redis, job.ID, "completed")
			if err := delivery.Ack(false); err != nil {
				log.Printf("ack job id=%s: %v", job.ID, err)
			}
		}
	}
}
