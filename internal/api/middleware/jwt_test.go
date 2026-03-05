package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/remnawave/node-go/internal/logger"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return privateKey, string(publicKeyPEM)
}

func generateTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return tokenString
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	claims := jwt.MapClaims{
		"sub":  "user123",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"role": "admin",
	}
	token := generateTestToken(t, privateKey, claims)

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		storedClaims, exists := c.Get("jwt_claims")
		if !exists {
			t.Error("Expected jwt_claims in context")
			c.Status(http.StatusInternalServerError)
			return
		}
		mapClaims, ok := storedClaims.(jwt.MapClaims)
		if !ok {
			t.Error("Expected jwt_claims to be MapClaims")
			c.Status(http.StatusInternalServerError)
			return
		}
		if mapClaims["sub"] != "user123" {
			t.Errorf("Expected sub=user123, got %v", mapClaims["sub"])
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if !handlerCalled.Load() {
		t.Error("Expected handler to be called for valid token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestJWTMiddleware_MissingAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for missing auth header")
	}
}

func TestJWTMiddleware_InvalidAuthFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "token123"},
		{"wrong prefix", "Basic token123"},
		{"empty bearer", "Bearer "},
		{"bearer only", "Bearer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var handlerCalled atomic.Bool
			router := gin.New()
			router.Use(JWTMiddleware(publicKeyPEM, log))
			router.GET("/test", func(c *gin.Context) {
				handlerCalled.Store(true)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if handlerCalled.Load() {
				t.Errorf("[%s] Expected handler NOT to be called", tc.name)
			}
		})
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for invalid token")
	}
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := generateTestToken(t, privateKey, claims)

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for expired token")
	}
}

func TestJWTMiddleware_WrongSigningKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey1, _ := generateTestKeyPair(t)
	_, publicKeyPEM2 := generateTestKeyPair(t)

	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestToken(t, privateKey1, claims)

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM2, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for wrong signing key")
	}
}

func TestJWTMiddleware_WrongSigningMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("secret"))

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for wrong signing method")
	}
}

func TestJWTMiddleware_InvalidPublicKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware("invalid-key", log))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer sometoken")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if handlerCalled.Load() {
		t.Error("Expected handler NOT to be called for invalid public key")
	}
}

func TestJWTMiddleware_BearerCaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := generateTestKeyPair(t)
	log := logger.New(logger.Config{Level: logger.LevelDebug, Format: logger.FormatPretty})

	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := generateTestToken(t, privateKey, claims)

	testCases := []string{"bearer", "BEARER", "Bearer", "bEaReR"}

	for _, prefix := range testCases {
		t.Run(prefix, func(t *testing.T) {
			var handlerCalled atomic.Bool
			router := gin.New()
			router.Use(JWTMiddleware(publicKeyPEM, log))
			router.GET("/test", func(c *gin.Context) {
				handlerCalled.Store(true)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", prefix+" "+token)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if !handlerCalled.Load() {
				t.Errorf("Expected handler to be called for prefix '%s'", prefix)
			}
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for prefix '%s', got %d", prefix, w.Code)
			}
		})
	}
}

func TestJWTMiddleware_NilLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := generateTestKeyPair(t)

	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := generateTestToken(t, privateKey, claims)

	var handlerCalled atomic.Bool
	router := gin.New()
	router.Use(JWTMiddleware(publicKeyPEM, nil))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled.Store(true)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if !handlerCalled.Load() {
		t.Error("Expected handler to be called with nil logger")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 with nil logger, got %d", w.Code)
	}
}

func TestParseRSAPublicKey_PKIX(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	key, err := ParseRSAPublicKey(string(publicKeyPEM))
	if err != nil {
		t.Errorf("Failed to parse PKIX public key: %v", err)
	}
	if key == nil {
		t.Error("Expected non-nil key")
	}
}

func TestParseRSAPublicKey_PKCS1(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	publicKeyBytes := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	key, err := ParseRSAPublicKey(string(publicKeyPEM))
	if err != nil {
		t.Errorf("Failed to parse PKCS1 public key: %v", err)
	}
	if key == nil {
		t.Error("Expected non-nil key")
	}
}

func TestParseRSAPublicKey_InvalidPEM(t *testing.T) {
	_, err := ParseRSAPublicKey("not a pem")
	if err == nil {
		t.Error("Expected error for invalid PEM")
	}
}

func TestParseRSAPublicKey_InvalidKey(t *testing.T) {
	invalidPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte("invalid key data"),
	})

	_, err := ParseRSAPublicKey(string(invalidPEM))
	if err == nil {
		t.Error("Expected error for invalid key data")
	}
}
