package app

import (
	"fmt"
	"net/http"

	"github.com/tokiou/agents-at-scale/internal/config"
)

func Run(cfg config.Config) error {
	server := &http.Server{
		Addr: cfg.Address,
	}

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}
