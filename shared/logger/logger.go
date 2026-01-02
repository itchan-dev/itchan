package logger

import (
	"log/slog"
	"os"
	"strings"
)

var Log *slog.Logger

func init() {
	// Auto-initialize with safe defaults for tests and development
	// Production code can override by calling Initialize() explicitly
	Initialize("info", false)
}

// Initialize sets up the global logger with the specified level and format
func Initialize(level string, useJSON bool) {
	var handler slog.Handler

	// Parse log level
	logLevel := parseLevel(level)

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true, // Equivalent to log.Lshortfile - adds file and line number
	}

	// Choose handler based on format
	if useJSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log) // Make it the default for entire program
}

// parseLevel converts string log level to slog.Level
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Default to Info if invalid level provided
		return slog.LevelInfo
	}
}
