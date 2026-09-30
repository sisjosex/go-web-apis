package utils

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogging installs the process logger (INFRA-004): one slog handler on stderr at LOG_LEVEL
// (debug|info|warn|error) in LOG_FORMAT (text|json). slog.SetDefault also routes the standard
// `log` package through it, so every existing log.Printf lands at INFO and a per-event line only
// has to move to slog.Debug to leave a production log alone.
func SetupLogging(level, format string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}
