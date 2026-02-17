package observability

import (
	"log/slog"
	"os"
	"strings"

	"github.com/polyshift/microkernel/internal/core/config"
)

func InitLogger(cfg config.ObservabilityConfig) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Logging.Level),
	}

	if strings.ToLower(cfg.Logging.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.Info("Logger initialized", "level", cfg.Logging.Level, "format", cfg.Logging.Format)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
