package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Default is the application-wide logger. Call Setup() to configure it.
var Default = slog.Default()

// Setup initializes the global logger with the given format and level.
// format: "text" (human-readable) or "json" (structured).
// level: "debug", "info", "warn", "error".
func Setup(format, level string) {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	Default = slog.New(handler)
	slog.SetDefault(Default)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
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
