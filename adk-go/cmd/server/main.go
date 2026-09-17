package main

import (
	"os"

	"github.com/tokiou/agents-at-scale/internal/app"
	"github.com/tokiou/agents-at-scale/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.Run(cfg); err != nil {
		// app.Run logs the failure through the configured structured logger.
		os.Exit(1)
	}
}
