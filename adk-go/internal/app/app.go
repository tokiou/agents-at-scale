package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokiou/agents-at-scale/internal/config"
	"github.com/tokiou/agents-at-scale/internal/health"
	"github.com/tokiou/agents-at-scale/internal/jobs"
	platformlogger "github.com/tokiou/agents-at-scale/internal/platform/logger"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres"
	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
)

func Run(cfg config.Config) error {
	appLogger := platformlogger.New(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(appLogger)
	appLogger.Info("application starting", "address", cfg.Address)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
	if err != nil {
		appLogger.Error("postgres initialization failed", "error", err)
		return err
	}
	defer db.Close()
	redisClient, err := redisplatform.New(ctx, cfg.RedisURL)
	if err != nil {
		appLogger.Error("redis initialization failed", "error", err)
		return err
	}
	defer redisClient.Close()
	rabbitClient, err := rabbitmq.New(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		appLogger.Error("rabbitmq initialization failed", "error", err)
		return err
	}
	defer rabbitClient.Close()

	jobService := jobs.New(redisClient, rabbitClient)
	if err := jobService.Start(ctx); err != nil {
		appLogger.Error("job service failed to start", "error", err)
		return err
	}

	jobHandler := jobs.NewHandler(jobService)
	healthHandler := health.NewHandler()
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: NewRouter(healthHandler, jobHandler),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			appLogger.Info("application stopped")
			return nil
		}
		appLogger.Error("http server failed", "error", err)
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}
