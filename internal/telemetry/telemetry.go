//go:build linux
// +build linux

package telemetry

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func Init() error {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = ""

	// Debug level for development
	if os.Getenv("THRIVE_LOG_LEVEL") == "debug" {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

func Logger() *zap.Logger {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
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
