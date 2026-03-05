package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/hteppl/remnawave-node-go/internal/errors"
)

func TestErrors_AllCodesPresent(t *testing.T) {
	expectedCodes := []string{
		"A001", "A002", "A003", "A004", "A005", "A006",
		"A009", "A010", "A011", "A012", "A013", "A014",
		"A015", "A016", "A017",
	}

	for _, code := range expectedCodes {
		t.Run(code, func(t *testing.T) {
			e, ok := apperrors.GetError(code)
			assert.True(t, ok, "error code %s should exist", code)
			assert.Equal(t, code, e.Code)
			assert.NotEmpty(t, e.Message)
			assert.Greater(t, e.HTTPCode, 0)
		})
	}
}

func TestErrors_HTTPCodes(t *testing.T) {
	tests := []struct {
		code     string
		httpCode int
	}{
		{"A001", 500},
		{"A003", 401},
		{"A004", 403},
		{"A010", 500},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			e, ok := apperrors.GetError(tt.code)
			assert.True(t, ok)
			assert.Equal(t, tt.httpCode, e.HTTPCode)
		})
	}
}

func TestGetError_NotFound(t *testing.T) {
	_, ok := apperrors.GetError("INVALID")
	assert.False(t, ok)
}

func TestErrorConstants(t *testing.T) {
	assert.Equal(t, "A001", apperrors.CodeInternalServerError)
	assert.Equal(t, "A003", apperrors.CodeUnauthorized)
	assert.Equal(t, "A010", apperrors.CodeFailedToGetSystemStats)
}
