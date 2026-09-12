package config

import (
	"os"
	"strconv"
)

type Config struct {
	Address         string
	DatabaseURL     string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

func Load() Config {
	address := os.Getenv("AGENTS_SERVER_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	return Config{
		Address:         address,
		DatabaseURL:     envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agents_at_scale?sslmode=disable"),
		MaxOpenConns:    envIntOrDefault("DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns:    envIntOrDefault("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: envIntOrDefault("DB_CONN_MAX_LIFETIME_MINUTES", 30),
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
