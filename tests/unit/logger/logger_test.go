package logger_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/logger"
)

func TestNew_DefaultsToInfoLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.Debug("debug message")
	log.Info("info message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.Contains(t, output, "info message")
}

func TestLogger_DebugLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelDebug,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.Debug("debug message")
	log.Info("info message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "info message")
}

func TestLogger_WarnLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelWarn,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestLogger_ErrorLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelError,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestLogger_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.Info("test message")

	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test message", entry["message"])
	assert.Contains(t, entry, "time")
}

func TestLogger_PrettyFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Output: buf,
		Format: logger.FormatPretty,
	})

	log.Info("pretty message")

	output := buf.String()
	assert.Contains(t, output, "pretty message")
	assert.Contains(t, output, "INF")
}

func TestLogger_WithField(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.WithField("key", "value").Info("message with field")

	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "value", entry["key"])
}

func TestLogger_WithError(t *testing.T) {
	buf := &bytes.Buffer{}
	log := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Output: buf,
		Format: logger.FormatJSON,
	})

	log.WithError(assert.AnError).Error("error occurred")

	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Contains(t, entry, "error")
}

func TestLogger_LevelStrings(t *testing.T) {
	tests := []struct {
		level    logger.Level
		expected string
	}{
		{logger.LevelDebug, "debug"},
		{logger.LevelInfo, "info"},
		{logger.LevelWarn, "warn"},
		{logger.LevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			buf := &bytes.Buffer{}
			log := logger.New(logger.Config{
				Level:  tt.level,
				Output: buf,
				Format: logger.FormatJSON,
			})

			switch tt.level {
			case logger.LevelDebug:
				log.Debug("msg")
			case logger.LevelInfo:
				log.Info("msg")
			case logger.LevelWarn:
				log.Warn("msg")
			case logger.LevelError:
				log.Error("msg")
			}

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Len(t, lines, 1)

			var entry map[string]interface{}
			err := json.Unmarshal([]byte(lines[0]), &entry)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, entry["level"])
		})
	}
}

func TestLogger_Zerolog_ReturnsUnderlyingLogger(t *testing.T) {
	log := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Format: logger.FormatJSON,
	})

	zl := log.Zerolog()
	assert.NotNil(t, zl)
}
