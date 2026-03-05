package middleware

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/hteppl/remnawave-node-go/internal/logger"
)

func JWTMiddleware(publicKeyPEM string, log *logger.Logger) gin.HandlerFunc {
	publicKey, err := ParseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return func(c *gin.Context) {
			if log != nil {
				log.Error(fmt.Sprintf("JWT middleware disabled: invalid public key: %v", err))
			}
			destroySocket(c)
		}
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logAuthFailure(log, c, "missing Authorization header")
			destroySocket(c)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			logAuthFailure(log, c, "invalid Authorization header format")
			destroySocket(c)
			return
		}

		tokenString := parts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return publicKey, nil
		}, jwt.WithValidMethods([]string{"RS256"}))

		if err != nil {
			logAuthFailure(log, c, fmt.Sprintf("token validation failed: %v", err))
			destroySocket(c)
			return
		}

		if !token.Valid {
			logAuthFailure(log, c, "invalid token")
			destroySocket(c)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("jwt_claims", claims)
		}

		c.Next()
	}
}

func ParseRSAPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		return rsaPub, nil
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

func logAuthFailure(log *logger.Logger, c *gin.Context, reason string) {
	if log != nil {
		log.WithField("url", c.Request.URL.String()).
			WithField("ip", c.ClientIP()).
			WithField("reason", reason).
			Error("Incorrect SECRET_KEY or JWT! Request dropped.")
	}
}

func destroySocket(c *gin.Context) {
	defer func() {
		_ = recover()
		c.Abort()
	}()

	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	conn.Close()
}
