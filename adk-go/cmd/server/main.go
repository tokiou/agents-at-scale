package main

import (
	"log"

	"github.com/tokiou/agents-at-scale/internal/app"
	"github.com/tokiou/agents-at-scale/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
