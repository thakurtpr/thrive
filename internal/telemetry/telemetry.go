//go:build linux
// +build linux

package telemetry

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger   *zap.Logger
	initOnce sync.Once
)

// Init configures the global logger. Should be called once at startup before
// any goroutines that call Logger(). Subsequent calls are no-ops.
func Init() error {
	var buildErr error
	initOnce.Do(func() {
		config := zap.NewProductionConfig()
		config.OutputPaths = []string{"stdout"}
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.StacktraceKey = ""

		if os.Getenv("THRIVE_LOG_LEVEL") == "debug" {
			config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}

		logger, buildErr = config.Build()
	})
	return buildErr
}

// Logger returns the global logger. If Init has not been called, a default
// production logger is initialised exactly once. Safe for concurrent use.
func Logger() *zap.Logger {
	initOnce.Do(func() {
		logger, _ = zap.NewProduction()
	})
	return logger
}

func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Logger().Fatal(msg, fields...)
	os.Exit(1)
}

func WithContext(ctx context.Context, fields ...zap.Field) []zap.Field {
	return fields
}

func FieldString(key string, val string) zap.Field {
	return zap.String(key, val)
}

func FieldInt(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func FieldInt64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func FieldError(err error) zap.Field {
	return zap.Error(err)
}

func FieldBool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}
