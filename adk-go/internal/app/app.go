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

	"github.com/tokiou/agents-at-scale/internal/agent"
	"github.com/tokiou/agents-at-scale/internal/airline"
	airlinerepository "github.com/tokiou/agents-at-scale/internal/airline/repository"
	"github.com/tokiou/agents-at-scale/internal/config"
	"github.com/tokiou/agents-at-scale/internal/health"
	"github.com/tokiou/agents-at-scale/internal/jobs"
	platformlogger "github.com/tokiou/agents-at-scale/internal/platform/logger"
	"github.com/tokiou/agents-at-scale/internal/platform/openrouter"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres"
	"github.com/tokiou/agents-at-scale/internal/platform/rabbitmq"
	redisplatform "github.com/tokiou/agents-at-scale/internal/platform/redis"
	"github.com/tokiou/agents-at-scale/internal/runtime"
	"google.golang.org/adk/v2/session"
)

func Run(cfg config.Config) error {
	logger := platformlogger.New(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)
	logger.Info("application starting", "address", cfg.Address)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("postgres initialization failed", "error", err)
		return err
	}
	defer db.Close()
	queries := postgres.NewQueries(db)
	airlineService := airline.NewService(
		airlinerepository.NewCustomerRepository(queries),
		airlinerepository.NewReservationRepository(queries),
		airlinerepository.NewFlightRepository(queries),
		airlinerepository.NewTravelCreditRepository(queries),
		airlinerepository.NewFlightChangeRepository(queries),
		airlinerepository.NewRebookingRepository(queries, db),
	)
	llm, err := openrouter.New(logger, openrouter.Config{
		Deployment: cfg.OpenRouterModel,
		APIKey:     cfg.OpenRouterAPIKey,
		BaseURL:    cfg.OpenRouterBaseURL,
	})
	if err != nil {
		logger.Error("openrouter initialization failed", "error", err)
		return err
	}
	rootAgent, err := agent.NewWithDependencies(logger, llm, airlineService)
	if err != nil {
		logger.Error("agent initialization failed", "error", err)
		return err
	}
	// ADK sessions are process-scoped for now; a restart loses pending
	// confirmations and sessions are not shared between replicas.
	agentRunner, err := runtime.NewAgentRunner(logger, rootAgent, session.InMemoryService())
	if err != nil {
		logger.Error("agent runner initialization failed", "error", err)
		return err
	}
	redisClient, err := redisplatform.New(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("redis initialization failed", "error", err)
		return err
	}
	defer redisClient.Close()
	rabbitClient, err := rabbitmq.New(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		logger.Error("rabbitmq initialization failed", "error", err)
		return err
	}
	defer rabbitClient.Close()

	jobService := jobs.New(logger, jobs.NewRedisStatusStore(redisClient), rabbitClient, agentRunner)
	if err := jobService.Start(ctx); err != nil {
		logger.Error("job service failed to start", "error", err)
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
			logger.Info("application stopped")
			return nil
		}
		logger.Error("http server failed", "error", err)
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}
