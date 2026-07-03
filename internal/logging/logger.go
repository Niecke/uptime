package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"niecke-it.de/uptime/internal/version"
)

func New(levelStr string) *slog.Logger {
	level, err := parseLevel(levelStr)
	if err != nil {
		// Fall back to INFO and log the problem once the logger exists
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	// version and git_hash are attached here so every log record carries them by default
	return slog.New(handler).With("version", version.Version, "git_hash", version.GitHash)
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}
