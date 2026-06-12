package logger

import (
	"encoding/json"
	"fmt"
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

// FormatSQLArgs renders bind parameters for logs ($1=..., $2=...).
func FormatSQLArgs(args []any) string {
	if len(args) == 0 {
		return "[]"
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("$%d=%s", i+1, formatSQLArg(a))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatSQLArg(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		return jsonQuote(x)
	case []byte:
		return jsonQuote(string(x))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

func jsonQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return string(b)
}
