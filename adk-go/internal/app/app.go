package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokiou/agents-at-scale/internal/config"
	"github.com/tokiou/agents-at-scale/internal/jobs"
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

	jobService := jobs.New(redisClient, rabbitClient)
	if err := jobService.Start(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.Handle("/jobs", jobService.Handler())
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

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
