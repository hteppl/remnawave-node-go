package config_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/config"
)

func makeValidSecretKey() string {
	payload := map[string]string{
		"caCertPem":    "-----BEGIN CERTIFICATE-----\nCA\n-----END CERTIFICATE-----",
		"jwtPublicKey": "-----BEGIN PUBLIC KEY-----\nJWT\n-----END PUBLIC KEY-----",
		"nodeCertPem":  "-----BEGIN CERTIFICATE-----\nNODE\n-----END CERTIFICATE-----",
		"nodeKeyPem":   "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----",
	}
	data, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(data)
}

func TestParseSecretKey_Success(t *testing.T) {
	secretKey := makeValidSecretKey()

	payload, err := config.ParseSecretKey(secretKey)
	require.NoError(t, err)

	assert.Contains(t, payload.CACertPEM, "CA")
	assert.Contains(t, payload.JWTPublicKey, "JWT")
	assert.Contains(t, payload.NodeCertPEM, "NODE")
	assert.Contains(t, payload.NodeKeyPEM, "KEY")
}

func TestParseSecretKey_Empty(t *testing.T) {
	_, err := config.ParseSecretKey("")
	assert.True(t, errors.Is(err, config.ErrSecretKeyEmpty))
}

func TestParseSecretKey_InvalidBase64(t *testing.T) {
	_, err := config.ParseSecretKey("not-valid-base64!!!")
	assert.True(t, errors.Is(err, config.ErrSecretKeyInvalidBase64))
}

func TestParseSecretKey_InvalidJSON(t *testing.T) {
	notJSON := base64.StdEncoding.EncodeToString([]byte("not json"))
	_, err := config.ParseSecretKey(notJSON)
	assert.True(t, errors.Is(err, config.ErrSecretKeyInvalidJSON))
}

func TestParseSecretKey_MissingField_CACertPem(t *testing.T) {
	payload := map[string]string{
		"jwtPublicKey": "jwt",
		"nodeCertPem":  "cert",
		"nodeKeyPem":   "key",
	}
	data, _ := json.Marshal(payload)
	secretKey := base64.StdEncoding.EncodeToString(data)

	_, err := config.ParseSecretKey(secretKey)
	assert.True(t, errors.Is(err, config.ErrSecretKeyMissingField))
	assert.Contains(t, err.Error(), "caCertPem")
}

func TestParseSecretKey_MissingField_JWTPublicKey(t *testing.T) {
	payload := map[string]string{
		"caCertPem":   "ca",
		"nodeCertPem": "cert",
		"nodeKeyPem":  "key",
	}
	data, _ := json.Marshal(payload)
	secretKey := base64.StdEncoding.EncodeToString(data)

	_, err := config.ParseSecretKey(secretKey)
	assert.True(t, errors.Is(err, config.ErrSecretKeyMissingField))
	assert.Contains(t, err.Error(), "jwtPublicKey")
}

func TestParseSecretKey_MissingField_NodeCertPem(t *testing.T) {
	payload := map[string]string{
		"caCertPem":    "ca",
		"jwtPublicKey": "jwt",
		"nodeKeyPem":   "key",
	}
	data, _ := json.Marshal(payload)
	secretKey := base64.StdEncoding.EncodeToString(data)

	_, err := config.ParseSecretKey(secretKey)
	assert.True(t, errors.Is(err, config.ErrSecretKeyMissingField))
	assert.Contains(t, err.Error(), "nodeCertPem")
}

func TestParseSecretKey_MissingField_NodeKeyPem(t *testing.T) {
	payload := map[string]string{
		"caCertPem":    "ca",
		"jwtPublicKey": "jwt",
		"nodeCertPem":  "cert",
	}
	data, _ := json.Marshal(payload)
	secretKey := base64.StdEncoding.EncodeToString(data)

	_, err := config.ParseSecretKey(secretKey)
	assert.True(t, errors.Is(err, config.ErrSecretKeyMissingField))
	assert.Contains(t, err.Error(), "nodeKeyPem")
}
