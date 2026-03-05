package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestSecretKey() string {
	payload := map[string]string{
		"caCertPem":    "ca-cert",
		"jwtPublicKey": "jwt-key",
		"nodeCertPem":  "node-cert",
		"nodeKeyPem":   "node-key",
	}
	data, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(data)
}

func TestLoad_FromEnvOnly(t *testing.T) {
	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Unsetenv("CONFIG_PATH")
	os.Unsetenv("NODE_PORT")
	os.Unsetenv("INTERNAL_REST_PORT")
	os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("SECRET_KEY")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, DefaultNodePort, cfg.NodePort)
	assert.Equal(t, DefaultInternalRestPort, cfg.InternalRestPort)
	assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
	assert.NotNil(t, cfg.Payload)
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("NODE_PORT", "3333")
	os.Setenv("INTERNAL_REST_PORT", "62000")
	os.Setenv("LOG_LEVEL", "debug")
	os.Unsetenv("CONFIG_PATH")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("NODE_PORT")
		os.Unsetenv("INTERNAL_REST_PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 3333, cfg.NodePort)
	assert.Equal(t, 62000, cfg.InternalRestPort)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_MissingSecretKey(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	os.Unsetenv("CONFIG_PATH")

	_, err := Load()
	assert.ErrorIs(t, err, ErrConfigSecretKeyRequired)
}

func TestLoad_InvalidSecretKey(t *testing.T) {
	os.Setenv("SECRET_KEY", "invalid-base64!!!")
	os.Unsetenv("CONFIG_PATH")
	defer os.Unsetenv("SECRET_KEY")

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_FromConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := map[string]interface{}{
		"nodePort":         4444,
		"internalRestPort": 63000,
		"logLevel":         "warn",
	}
	data, _ := json.Marshal(configData)
	_ = os.WriteFile(configPath, data, 0644)

	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("CONFIG_PATH", configPath)
	os.Unsetenv("NODE_PORT")
	os.Unsetenv("INTERNAL_REST_PORT")
	os.Unsetenv("LOG_LEVEL")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("CONFIG_PATH")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 4444, cfg.NodePort)
	assert.Equal(t, 63000, cfg.InternalRestPort)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoad_EnvOverridesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := map[string]interface{}{
		"nodePort": 4444,
		"logLevel": "warn",
	}
	data, _ := json.Marshal(configData)
	_ = os.WriteFile(configPath, data, 0644)

	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("CONFIG_PATH", configPath)
	os.Setenv("NODE_PORT", "5555")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("CONFIG_PATH")
		os.Unsetenv("NODE_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 5555, cfg.NodePort)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoad_InvalidPortIgnored(t *testing.T) {
	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("NODE_PORT", "not-a-number")
	os.Unsetenv("CONFIG_PATH")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("NODE_PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, DefaultNodePort, cfg.NodePort)
}
