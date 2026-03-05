package config_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/config"
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
	os.Unsetenv("NODE_PORT")
	os.Unsetenv("INTERNAL_REST_PORT")
	os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("SECRET_KEY")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, config.DefaultNodePort, cfg.NodePort)
	assert.Equal(t, config.DefaultInternalRestPort, cfg.InternalRestPort)
	assert.Equal(t, config.DefaultLogLevel, cfg.LogLevel)
	assert.NotNil(t, cfg.Payload)
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("NODE_PORT", "3333")
	os.Setenv("INTERNAL_REST_PORT", "62000")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("NODE_PORT")
		os.Unsetenv("INTERNAL_REST_PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 3333, cfg.NodePort)
	assert.Equal(t, 62000, cfg.InternalRestPort)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_MissingSecretKey(t *testing.T) {
	os.Unsetenv("SECRET_KEY")

	_, err := config.Load()
	assert.ErrorIs(t, err, config.ErrConfigSecretKeyRequired)
}

func TestLoad_InvalidSecretKey(t *testing.T) {
	os.Setenv("SECRET_KEY", "invalid-base64!!!")
	defer os.Unsetenv("SECRET_KEY")

	_, err := config.Load()
	assert.Error(t, err)
}

func TestLoad_InvalidPortIgnored(t *testing.T) {
	os.Setenv("SECRET_KEY", makeTestSecretKey())
	os.Setenv("NODE_PORT", "not-a-number")
	defer func() {
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("NODE_PORT")
	}()

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, config.DefaultNodePort, cfg.NodePort)
}
