// Package utils provides utility functions for the loan eligibility engine.
package utils

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the global logger instance.
var Logger *zap.Logger

// InitLogger initializes the global logger.
func InitLogger(level string) error {
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Check if we're running in Lambda
	isLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""

	var config zap.Config
	if isLambda {
		// Production config for Lambda (JSON output)
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapLevel)
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	} else {
		// Development config for local testing
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zapLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var err error
	Logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// GetLogger returns the global logger, initializing if necessary.
func GetLogger() *zap.Logger {
	if Logger == nil {
		_ = InitLogger("info")
	}
	return Logger
}

// Sync flushes any buffered log entries.
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// LogField creates a zap field for structured logging.
type LogField = zap.Field

// Common field constructors
var (
	String   = zap.String
	Int      = zap.Int
	Int64    = zap.Int64
	Float64  = zap.Float64
	Bool     = zap.Bool
	Error    = zap.Error
	Any      = zap.Any
	Duration = zap.Duration
)
