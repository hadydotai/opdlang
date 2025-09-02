package logging

import (
	"io"
	"log/slog"
	"os"
)

type LogLevel string

const (
	LogLevelNone  LogLevel = "none"
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
)

var logger *slog.Logger

func Setup(optslevel LogLevel) {
	sink := io.Discard
	if optslevel != LogLevelNone {
		sink = os.Stderr
	}

	level := slog.LevelDebug
	if optslevel == LogLevelInfo {
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(sink, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
}
