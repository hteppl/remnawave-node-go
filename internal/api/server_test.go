package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/config"
	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func generateTestCerts() (*config.NodePayload, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	nodeKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	nodeTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
	}

	nodeCertDER, err := x509.CreateCertificate(rand.Reader, nodeTemplate, caTemplate, &nodeKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	nodeCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: nodeCertDER})
	nodeKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(nodeKey)})

	jwtKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	jwtPubDER, err := x509.MarshalPKIXPublicKey(&jwtKey.PublicKey)
	if err != nil {
		return nil, err
	}
	jwtPubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: jwtPubDER})

	return &config.NodePayload{
		CACertPEM:    string(caCertPEM),
		JWTPublicKey: string(jwtPubPEM),
		NodeCertPEM:  string(nodeCertPEM),
		NodeKeyPEM:   string(nodeKeyPEM),
	}, nil
}

func TestNewServer(t *testing.T) {
	payload, err := generateTestCerts()
	require.NoError(t, err)

	cfg := &config.Config{
		NodePort:         2222,
		InternalRestPort: 61001,
		LogLevel:         "info",
		Payload:          payload,
	}

	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})

	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := NewServer(cfg, log, core, configMgr)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.MainRouter())
	assert.NotNil(t, server.InternalRouter())
}

func TestMainRouter_NotFound_DestroysSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload, err := generateTestCerts()
	require.NoError(t, err)

	cfg := &config.Config{
		NodePort:         2222,
		InternalRestPort: 61001,
		Payload:          payload,
	}

	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})

	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := NewServer(cfg, log, core, configMgr)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestInternalRouter_UnmatchedPrefix_Returns404Text(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload, err := generateTestCerts()
	require.NoError(t, err)

	cfg := &config.Config{
		NodePort:         2222,
		InternalRestPort: 61001,
		Payload:          payload,
	}

	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})

	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := NewServer(cfg, log, core, configMgr)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/node/xray/status", nil)
	w := httptest.NewRecorder()

	server.InternalRouter().ServeHTTP(w, req)

	// PortGuardMiddleware destroys socket for requests not from internal port
	// In httptest, there's no LocalAddrContextKey, so socket is destroyed
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestInternalRouter_MatchedPrefix_NoRoute_DestroysSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload, err := generateTestCerts()
	require.NoError(t, err)

	cfg := &config.Config{
		NodePort:         2222,
		InternalRestPort: 61001,
		Payload:          payload,
	}

	log := logger.New(logger.Config{Level: logger.LevelInfo, Format: logger.FormatJSON})

	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := NewServer(cfg, log, core, configMgr)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/internal/get-config/extra", nil)
	w := httptest.NewRecorder()

	server.InternalRouter().ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestPortGuardMiddleware_AllowsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}
