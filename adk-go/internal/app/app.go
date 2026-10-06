package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	goruntime "runtime"
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
)

func Run(cfg config.Config) error {
	logger := platformlogger.New(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)
	logger.Info("application starting", "address", cfg.Address, "gomaxprocs", goruntime.GOMAXPROCS(0))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.PprofAddress != "" {
		// Profiling for benchmark diagnosis only; disabled unless PPROF_ADDR is set.
		go func() {
			logger.Info("pprof listening", "address", cfg.PprofAddress)
			if err := http.ListenAndServe(cfg.PprofAddress, nil); err != nil {
				logger.Error("pprof server failed", "error", err)
			}
		}()
	}

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
		Deployment:     cfg.OpenRouterModel,
		APIKey:         cfg.OpenRouterAPIKey,
		BaseURL:        cfg.OpenRouterBaseURL,
		MaxConnections: cfg.LLMMaxConnections,
		Timeout:        time.Duration(cfg.LLMTimeoutSeconds) * time.Second,
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
	sessionService, sessionDB, err := postgres.NewSessionService(postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	}, cfg.RunMigrations)
	if err != nil {
		logger.Error("session store initialization failed", "error", err)
		return err
	}
	defer sessionDB.Close()
	agentRunner, err := runtime.NewAgentRunner(logger, rootAgent, sessionService)
	if err != nil {
		logger.Error("agent runner initialization failed", "error", err)
		return err
	}
	redisClient, err := redisplatform.New(ctx, cfg.RedisURL, cfg.RedisPoolSize)
	if err != nil {
		logger.Error("redis initialization failed", "error", err)
		return err
	}
	defer redisClient.Close()
	rabbitClient, err := rabbitmq.New(cfg.RabbitMQURL, cfg.RabbitMQQueue, cfg.RetryDelays())
	if err != nil {
		logger.Error("rabbitmq initialization failed", "error", err)
		return err
	}
	defer rabbitClient.Close()

	jobService := jobs.New(logger, jobs.NewRedisStore(redisClient), rabbitClient, agentRunner, jobs.Options{
		Concurrency: cfg.WorkerConcurrency,
		MaxAttempts: cfg.JobMaxAttempts,
		LockTTL:     time.Duration(cfg.JobLockTTLMS) * time.Millisecond,
	})
	if err := jobService.Start(ctx); err != nil {
		logger.Error("job service failed to start", "error", err)
		return err
	}

	airlineHandler := airline.NewHandler(jobService.Publish)
	healthHandler := health.NewHandler()
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: NewRouter(healthHandler, airlineHandler),
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
