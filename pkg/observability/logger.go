package observability

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a JSON logger shared by all services.
func NewLogger(service string) *slog.Logger {
	level := slog.LevelInfo

	switch strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	return slog.New(handler).With("service", service)
}
