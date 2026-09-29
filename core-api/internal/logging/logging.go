// Package logging provides the application's structured logger.
//
// Every log line is JSON with a level and a message, so the VPS can ship logs
// without a parsing layer. Request-scoped fields (request ID, and from M1 the
// family and user) are attached by middleware rather than by callers.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// ErrUnknownLevel is returned when LOG_LEVEL is not a recognised slog level.
var ErrUnknownLevel = fmt.Errorf("unknown log level")

// New builds the application logger from configuration.
func New(cfg config.Config) (*slog.Logger, error) {
	level, err := parseLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	return slog.New(handler).With(slog.String("service", "core-api")), nil
}

func parseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownLevel, name)
	}
}
