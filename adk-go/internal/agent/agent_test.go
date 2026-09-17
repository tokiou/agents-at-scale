package agent

import (
	"io"
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	if _, err := New(slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("New() error = %v", err)
	}
}
