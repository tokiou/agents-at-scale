package config

import (
	"os"
	"strconv"
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
	RabbitMQURL       string
	RabbitMQQueue     string
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
		RabbitMQURL:       envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQQueue:     envOrDefault("RABBITMQ_QUEUE", "agent_jobs"),
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
