package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog with service defaults.
type Logger struct {
	*slog.Logger
}

// New creates a JSON logger at the given level (debug|info|warn|error).
func New(level string) *Logger {
	lvl := parseLevel(level)
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return &Logger{slog.New(h)}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Default returns an info-level logger.
func Default() *Logger {
	return New("info")
}
