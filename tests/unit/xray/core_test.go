package xray_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func makeMinimalConfig() []byte {
	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "none",
		},
		"inbounds": []interface{}{},
		"outbounds": []interface{}{
			map[string]interface{}{
				"tag":      "direct",
				"protocol": "freedom",
			},
		},
	}
	data, _ := json.Marshal(cfg)
	return data
}

func makeInvalidConfig() []byte {
	return []byte(`{"invalid": true, "inbounds": "not-an-array"}`)
}

func makeInvalidJSON() []byte {
	return []byte(`{not valid json`)
}

func TestNewCore(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	assert.NotNil(t, c)
	assert.False(t, c.IsRunning())
	assert.Nil(t, c.Instance())
}

func TestCore_GetVersion(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	version := c.GetVersion()
	assert.NotEmpty(t, version)
}

func TestCore_StartStop(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	err := c.Start(makeMinimalConfig())
	require.NoError(t, err)
	assert.True(t, c.IsRunning())
	assert.NotNil(t, c.Instance())

	err = c.Stop()
	require.NoError(t, err)
	assert.False(t, c.IsRunning())
	assert.Nil(t, c.Instance())
}

func TestCore_StartInvalidConfig(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	err := c.Start(makeInvalidJSON())
	assert.Error(t, err)
	assert.False(t, c.IsRunning())
}

func TestCore_StopWhenNotRunning(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	err := c.Stop()
	assert.NoError(t, err)
}

func TestCore_Restart(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	err := c.Start(makeMinimalConfig())
	require.NoError(t, err)
	assert.True(t, c.IsRunning())

	err = c.Restart(makeMinimalConfig())
	require.NoError(t, err)
	assert.True(t, c.IsRunning())

	err = c.Stop()
	require.NoError(t, err)
}

func TestValidateConfig_Valid(t *testing.T) {
	err := xray.ValidateConfig(makeMinimalConfig())
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	err := xray.ValidateConfig(makeInvalidJSON())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestValidateConfig_InvalidXrayConfig(t *testing.T) {
	cfg := []byte(`{"log": {"loglevel": "invalid-level-that-does-not-exist"}, "inbounds": [{"port": -1}]}`)
	err := xray.ValidateConfig(cfg)
	assert.Error(t, err)
}

func TestCore_DoubleStart(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})
	c := xray.NewCore(log)

	err := c.Start(makeMinimalConfig())
	require.NoError(t, err)
	defer func() { _ = c.Stop() }()

	err = c.Start(makeMinimalConfig())
	require.NoError(t, err)
	assert.True(t, c.IsRunning())
}

var _ = makeInvalidConfig
