package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address           string
	LogFormat         string
	LogLevel          string
	OpenRouterModel   string
	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	DatabaseURL       string
	RedisURL          string
	RedisPoolSize     int
	RabbitMQURL       string
	RabbitMQQueue     string
	WorkerConcurrency int
	JobMaxAttempts    int
	JobRetryBaseMS    int
	JobLockTTLMS      int
	RunMigrations     bool
	LLMMaxConnections int
	LLMTimeoutSeconds int
	PprofAddress      string
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   int
}

func Load() Config {
	address := os.Getenv("AGENTS_SERVER_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	return Config{
		Address:           address,
		LogFormat:         envOrDefault("LOG_FORMAT", "text"),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		OpenRouterModel:   envOrDefault("OPENROUTER_DEPLOYMENT", ""),
		OpenRouterAPIKey:  envOrDefault("OPENROUTER_API_KEY", ""),
		OpenRouterBaseURL: envOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		DatabaseURL:       envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agents_at_scale?sslmode=disable"),
		RedisURL:          envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		RedisPoolSize:     envIntOrDefault("REDIS_POOL_SIZE", 50),
		RabbitMQURL:       envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQQueue:     envOrDefault("RABBITMQ_QUEUE", "agent_jobs"),
		WorkerConcurrency: envIntOrDefault("WORKER_CONCURRENCY", 1),
		JobMaxAttempts:    envIntOrDefault("JOB_MAX_ATTEMPTS", 3),
		JobRetryBaseMS:    envIntOrDefault("JOB_RETRY_BASE_MS", 1000),
		JobLockTTLMS:      envIntOrDefault("JOB_LOCK_TTL_MS", 300000),
		RunMigrations:     envOrDefault("RUN_MIGRATIONS", "true") == "true",
		LLMMaxConnections: envIntOrDefault("LLM_MAX_CONNECTIONS", 100),
		LLMTimeoutSeconds: envIntOrDefault("LLM_TIMEOUT_SECONDS", 60),
		PprofAddress:      envOrDefault("PPROF_ADDR", ""),
		MaxOpenConns:      envIntOrDefault("DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns:      envIntOrDefault("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime:   envIntOrDefault("DB_CONN_MAX_LIFETIME_MINUTES", 30),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

// RetryDelays returns one exponential backoff delay per retry level:
// base, 2*base, 4*base... for JobMaxAttempts-1 retries (at least one level,
// which also delays jobs whose conversation is busy).
func (c Config) RetryDelays() []time.Duration {
	levels := max(c.JobMaxAttempts-1, 1)
	delays := make([]time.Duration, levels)
	for i := range delays {
		delays[i] = time.Duration(c.JobRetryBaseMS) * time.Millisecond << i
	}
	return delays
}
