package api_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/api"
)

func TestNewSuccessResponse(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := api.NewSuccessResponse(data)

	assert.Equal(t, data, resp.Response)
}

func TestNewSuccessResponse_JSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := api.NewSuccessResponse(data)

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)

	assert.Contains(t, parsed, "response")
	responseData := parsed["response"].(map[string]interface{})
	assert.Equal(t, "value", responseData["key"])
}

func TestNewErrorResponse(t *testing.T) {
	resp := api.NewErrorResponse("/node/xray/start", "Server error", "A001")

	assert.Equal(t, "/node/xray/start", resp.Path)
	assert.Equal(t, "Server error", resp.Message)
	assert.Equal(t, "A001", resp.ErrorCode)
	assert.NotEmpty(t, resp.Timestamp)

	_, err := time.Parse(time.RFC3339Nano, resp.Timestamp)
	assert.NoError(t, err)
}

func TestNewErrorResponse_JSON(t *testing.T) {
	resp := api.NewErrorResponse("/node/xray/start", "Server error", "A001")

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "/node/xray/start", parsed["path"])
	assert.Equal(t, "Server error", parsed["message"])
	assert.Equal(t, "A001", parsed["errorCode"])
	assert.Contains(t, parsed, "timestamp")
}

func TestNewValidationErrorResponse(t *testing.T) {
	errs := []api.ValidationError{
		{Path: []string{"field1"}, Message: "Required"},
		{Path: []string{"nested", "field2"}, Message: "Must be string"},
	}

	resp := api.NewValidationErrorResponse(errs)

	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "Validation failed", resp.Message)
	assert.Len(t, resp.Errors, 2)
}

func TestNewValidationErrorResponse_JSON(t *testing.T) {
	errs := []api.ValidationError{
		{Path: []string{"field"}, Message: "Required"},
	}

	resp := api.NewValidationErrorResponse(errs)

	jsonBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, float64(400), parsed["statusCode"])
	assert.Equal(t, "Validation failed", parsed["message"])
	assert.Contains(t, parsed, "errors")
}
