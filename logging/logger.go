package logging

func Log(level LogLevel, msg string, args ...any) {
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

func LogErr(err error, msg string) {
	if err == nil || logger == nil {
		return
	}

	logger.Error(msg, "error", err.Error())
}
