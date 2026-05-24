package log

import (
	"strings"

	"charm.land/log/v2"
)

type Logger interface {
	Debug(msg any, keyvals ...any)
	Debugf(format string, args ...any)
	Error(msg any, keyvals ...any)
	Errorf(format string, args ...any)
	Fatal(msg any, keyvals ...any)
	Fatalf(format string, args ...any)
	Info(msg any, keyvals ...any)
	Infof(format string, args ...any)
	Log(level log.Level, msg any, keyvals ...any)
	Logf(level log.Level, format string, args ...any)
	Print(msg any, keyvals ...any)
	Printf(format string, args ...any)
	Warn(msg any, keyvals ...any)
	Warnf(format string, args ...any)
	With(keyvals ...any) Logger
	WithPrefix(prefix string) Logger
	Indent(prefix string) Logger
	Unindent(prefix string) Logger
}

// A wrapper to wrap [log.Logger] as [Logger] to be used with [MultiLogger]
type wrapper struct {
	logger *log.Logger
}

func Wrap(l *log.Logger) Logger {
	return wrapper{logger: l}
}

func (w wrapper) Debug(msg any, keyvals ...any) {
	w.logger.Debug(msg, keyvals...)
}

func (w wrapper) Debugf(format string, args ...any) {
	w.logger.Debugf(format, args...)
}

func (w wrapper) Error(msg any, keyvals ...any) {
	w.logger.Error(msg, keyvals...)
}

func (w wrapper) Errorf(format string, args ...any) {
	w.logger.Errorf(format, args...)
}

func (w wrapper) Fatal(msg any, keyvals ...any) {
	w.logger.Fatal(msg, keyvals...)
}

func (w wrapper) Fatalf(format string, args ...any) {
	w.logger.Fatalf(format, args...)
}

func (w wrapper) Info(msg any, keyvals ...any) {
	w.logger.Info(msg, keyvals...)
}

func (w wrapper) Infof(format string, args ...any) {
	w.logger.Infof(format, args...)
}

func (w wrapper) Log(level log.Level, msg any, keyvals ...any) {
	w.logger.Log(level, msg, keyvals...)
}

func (w wrapper) Logf(level log.Level, format string, args ...any) {
	w.logger.Logf(level, format, args...)
}

func (w wrapper) Print(msg any, keyvals ...any) {
	w.logger.Print(msg, keyvals...)
}

func (w wrapper) Printf(format string, args ...any) {
	w.logger.Printf(format, args...)
}

func (w wrapper) Warn(msg any, keyvals ...any) {
	w.logger.Warn(msg, keyvals...)
}

func (w wrapper) Warnf(format string, args ...any) {
	w.logger.Warnf(format, args...)
}

func (w wrapper) With(keyvals ...any) Logger {
	return Wrap(w.logger.With(keyvals...))
}

func (w wrapper) WithPrefix(prefix string) Logger {
	return Wrap(w.logger.WithPrefix(prefix))
}

func (w wrapper) Indent(prefix string) Logger {
	return Wrap(w.logger.WithPrefix(w.logger.GetPrefix() + prefix))
}

func (w wrapper) Unindent(prefix string) Logger {
	return Wrap(w.logger.WithPrefix(strings.TrimSuffix(w.logger.GetPrefix(), prefix)))
}

// Compose multiple [Logger] into one [Logger]
type multiLogger struct {
	Loggers []Logger
}

func MultiLogger(loggers ...Logger) Logger {
	list := make([]Logger, 0, len(loggers))
	for _, l := range loggers {
		if l != nil {
			list = append(list, l)
		}
	}
	return multiLogger{
		Loggers: list,
	}
}

func (ml multiLogger) Debug(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Debug(msg, keyvals...)
	}
}

func (ml multiLogger) Debugf(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Debugf(format, args...)
	}
}

func (ml multiLogger) Error(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Error(msg, keyvals...)
	}
}

func (ml multiLogger) Errorf(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Errorf(format, args...)
	}
}

func (ml multiLogger) Fatal(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Fatal(msg, keyvals...)
	}
}

func (ml multiLogger) Fatalf(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Fatalf(format, args...)
	}
}

func (ml multiLogger) Info(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Info(msg, keyvals...)
	}
}

func (ml multiLogger) Infof(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Infof(format, args...)
	}
}

func (ml multiLogger) Log(level log.Level, msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Log(level, msg, keyvals...)
	}
}

func (ml multiLogger) Logf(level log.Level, format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Logf(level, format, args...)
	}
}

func (ml multiLogger) Print(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Print(msg, keyvals...)
	}
}

func (ml multiLogger) Printf(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Printf(format, args...)
	}
}

func (ml multiLogger) Warn(msg any, keyvals ...any) {
	for _, logger := range ml.Loggers {
		logger.Warn(msg, keyvals...)
	}
}

func (ml multiLogger) Warnf(format string, args ...any) {
	for _, logger := range ml.Loggers {
		logger.Warnf(format, args...)
	}
}

func (ml multiLogger) apply(f func(Logger) Logger) multiLogger {
	list := make([]Logger, len(ml.Loggers))
	for i, logger := range ml.Loggers {
		list[i] = f(logger)
	}
	return multiLogger{
		Loggers: list,
	}
}

func (ml multiLogger) With(keyvals ...any) Logger {
	return ml.apply(func(l Logger) Logger {
		return l.With(keyvals...)
	})
}

func (ml multiLogger) WithPrefix(prefix string) Logger {
	return ml.apply(func(l Logger) Logger {
		return l.WithPrefix(prefix)
	})
}

func (ml multiLogger) Indent(prefix string) Logger {
	return ml.apply(func(l Logger) Logger {
		return l.Indent(prefix)
	})
}

func (ml multiLogger) Unindent(prefix string) Logger {
	return ml.apply(func(l Logger) Logger {
		return l.Unindent(prefix)
	})
}
