package logger

import (
	"log/slog"
	"testing"
)

func TestNewConfiguresLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  slog.Level
	}{
		{name: "debug", level: "debug", want: slog.LevelDebug},
		{name: "warning alias", level: "warning", want: slog.LevelWarn},
		{name: "error", level: "error", want: slog.LevelError},
		{name: "default", level: "unknown", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New("json", tt.level)
			if !log.Enabled(nil, tt.want) {
				t.Fatalf("logger does not enable configured level %s", tt.want)
			}
			if tt.want > slog.LevelDebug && log.Enabled(nil, tt.want-1) {
				t.Fatalf("logger enabled level below configured threshold")
			}
		})
	}
}
