package main

import (
	"io"
	"log/slog"
	"os"
	"github.com/jessevdk/go-flags"
)

type LogLevel string

const (
	LogLevelNone  LogLevel = "none"
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
)

type Options struct {
	LogLevel LogLevel `short:"l" long:"loglevel" description:"Set the level of logging" choice:"none" choice:"info" choice:"debug" default:"info"`
}

var (
	opts        Options
	flagsparser = flags.NewParser(&opts, flags.Default)
	logger      *slog.Logger
)

func setupLogger() {
	sink := io.Discard
	if opts.LogLevel != LogLevelNone {
		sink = os.Stderr
	}

	level := slog.LevelDebug
	if opts.LogLevel == LogLevelInfo {
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(sink, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
}

func log(level LogLevel, msg string, args ...any) {
	if logger == nil {
		return
	}
	switch level {
	case LogLevelDebug:
		logger.Debug(msg, args...)
	case LogLevelInfo:
		logger.Info(msg, args...)
	default:
		panic("passing something else than Debug/Info, if you want to disable logging then call binary with -lnone or --loglevel=none")
	}
}

func main() {
	flagsparser.CommandHandler = func(command flags.Commander, args []string) error {
		setupLogger()
		return command.Execute(args)
	}

	if _, err := flagsparser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}

	if logger == nil {
		setupLogger()
	}
}
