package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new zap logger with the specified level and format
func NewLogger(level, format string) (*zap.Logger, error) {
	// Parse log level
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		return nil, fmt.Errorf("invalid log level: %s", level)
	}

	// Create config based on format
	var config zap.Config
	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set the log level
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	// Build the logger
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// NewDevelopmentLogger creates a development logger with console output
func NewDevelopmentLogger() (*zap.Logger, error) {
	return NewLogger("debug", "console")
}

// NewProductionLogger creates a production logger with JSON output
func NewProductionLogger() (*zap.Logger, error) {
	return NewLogger("info", "json")
}
