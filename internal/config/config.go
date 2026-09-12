package config

import "os"

type Config struct {
	Address string
}

func Load() Config {
	address := os.Getenv("AGENTS_SERVER_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	return Config{Address: address}
}
