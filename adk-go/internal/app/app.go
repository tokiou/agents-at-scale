package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tokiou/agents-at-scale/internal/config"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres"
)

func Run(cfg config.Config) error {
	db, err := postgres.New(context.Background(), postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	server := &http.Server{
		Addr: cfg.Address,
	}

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}
