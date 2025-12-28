package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger_ValidLevelsAndFormats(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"debug json", "debug", "json"},
		{"info json", "info", "json"},
		{"warn json", "warn", "json"},
		{"error json", "error", "json"},
		{"debug console", "debug", "console"},
		{"info console", "info", "console"},
		{"warn console", "warn", "console"},
		{"error console", "error", "console"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level, tt.format)

			require.NoError(t, err)
			require.NotNil(t, logger)

			// Verify logger can be used
			logger.Info("test log message")
		})
	}
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"invalid level", "invalid"},
		{"uppercase", "INFO"},
		{"trace level", "trace"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level, "json")

			assert.Error(t, err)
			assert.Nil(t, logger)
			assert.Contains(t, err.Error(), "invalid log level")
		})
	}
}

func TestNewLogger_Levels(t *testing.T) {
	tests := []struct {
		level    string
		zapLevel zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger, err := NewLogger(tt.level, "json")

			require.NoError(t, err)

			// Verify level is set correctly
			assert.True(t, logger.Core().Enabled(tt.zapLevel))
		})
	}
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger, err := NewDevelopmentLogger()

	require.NoError(t, err)
	require.NotNil(t, logger)

	// Development logger should be at debug level
	assert.True(t, logger.Core().Enabled(zapcore.DebugLevel))

	// Test that it works
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
}

func TestNewProductionLogger(t *testing.T) {
	logger, err := NewProductionLogger()

	require.NoError(t, err)
	require.NotNil(t, logger)

	// Production logger should be at info level
	assert.True(t, logger.Core().Enabled(zapcore.InfoLevel))
	// Debug should be disabled
	assert.False(t, logger.Core().Enabled(zapcore.DebugLevel))

	// Test that it works
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestNewLogger_JSONFormat(t *testing.T) {
	logger, err := NewLogger("info", "json")

	require.NoError(t, err)
	require.NotNil(t, logger)

	// JSON format uses production config
	logger.Info("test message", zap.String("key", "value"))
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	logger, err := NewLogger("info", "console")

	require.NoError(t, err)
	require.NotNil(t, logger)

	// Console format uses development config with colors
	logger.Info("test message", zap.String("key", "value"))
}

func TestLogger_LevelFiltering(t *testing.T) {
	// Create info-level logger
	logger, err := NewLogger("info", "json")
	require.NoError(t, err)

	// Info and above should be enabled
	assert.True(t, logger.Core().Enabled(zapcore.InfoLevel))
	assert.True(t, logger.Core().Enabled(zapcore.WarnLevel))
	assert.True(t, logger.Core().Enabled(zapcore.ErrorLevel))

	// Debug should be disabled
	assert.False(t, logger.Core().Enabled(zapcore.DebugLevel))
}

func TestLogger_ErrorLevel(t *testing.T) {
	// Create error-level logger
	logger, err := NewLogger("error", "json")
	require.NoError(t, err)

	// Only error level should be enabled
	assert.True(t, logger.Core().Enabled(zapcore.ErrorLevel))

	// Debug, info, warn should be disabled
	assert.False(t, logger.Core().Enabled(zapcore.DebugLevel))
	assert.False(t, logger.Core().Enabled(zapcore.InfoLevel))
	assert.False(t, logger.Core().Enabled(zapcore.WarnLevel))
}
