package log

import (
	"log/slog"
	"os"
	"strings"
)

// New creates a slog.Logger with the provided level (defaults to info).
func New(level string) *slog.Logger {
	lvl := ParseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// ParseLevel converts a string representation into a slog.Level.
func ParseLevel(level string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(level)) {
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
