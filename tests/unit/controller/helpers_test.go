package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/api/controller"
	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestWrapResponse(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	handlerCtrl := controller.NewHandlerController(core, configMgr, log)

	router := gin.New()
	group := router.Group("/node/handler")
	handlerCtrl.RegisterRoutes(group)

	// Call an endpoint that returns a wrapped response — get-inbound-users with valid JSON
	req := httptest.NewRequest("POST", "/node/handler/get-inbound-users", jsonBody(t, map[string]string{"tag": "test"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)
	_, hasResponse := raw["response"]
	assert.True(t, hasResponse, "response should be wrapped in 'response' field")
}

func TestWrapResponse_Nil(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	configMgr := xray.NewConfigManager(log)

	internalCtrl := controller.NewInternalController(configMgr, log)

	router := gin.New()
	group := router.Group("/internal")
	internalCtrl.RegisterRoutes(group)

	req := httptest.NewRequest("GET", "/internal/get-config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)
	_, hasResponse := raw["response"]
	assert.False(t, hasResponse, "internal get-config should NOT use response wrapper when no config set")
}

func TestStrPtr_NonEmpty(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	xrayCtrl := controller.NewXrayController(core, configMgr, log)

	router := gin.New()
	group := router.Group("/node/xray")
	xrayCtrl.RegisterRoutes(group)

	// Status endpoint uses strPtr for version — when not running, version should be nil
	req := httptest.NewRequest("GET", "/node/xray/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsRunning bool    `json:"isRunning"`
			Version   *string `json:"version"`
		} `json:"response"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.IsRunning)
	assert.NotNil(t, response.Response.Version, "version should always be returned")
}

func TestStrPtr_Empty(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	xrayCtrl := controller.NewXrayController(core, configMgr, log)

	router := gin.New()
	group := router.Group("/node/xray")
	xrayCtrl.RegisterRoutes(group)

	// Healthcheck uses strPtr — when not running, xrayVersion should be nil
	req := httptest.NewRequest("GET", "/node/xray/healthcheck", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			XrayVersion *string `json:"xrayVersion"`
		} `json:"response"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.XrayVersion, "xrayVersion should always be returned")
}
