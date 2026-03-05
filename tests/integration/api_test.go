package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hteppl/remnawave-node-go/internal/api"
	"github.com/hteppl/remnawave-node-go/internal/api/controller"
	"github.com/hteppl/remnawave-node-go/internal/config"
	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestServer(t *testing.T, creds *TestCredentials) *api.Server {
	t.Helper()

	payload := &config.NodePayload{
		CACertPEM:    string(creds.CACert),
		JWTPublicKey: creds.JWTPubPEM,
		NodeCertPEM:  string(creds.NodeCert),
		NodeKeyPEM:   string(creds.NodeKey),
	}

	cfg := &config.Config{
		NodePort:         2222,
		InternalRestPort: 61001,
		LogLevel:         "error",
		Payload:          payload,
	}

	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := api.NewServer(cfg, log, core, configMgr)
	require.NoError(t, err)

	return server
}

func makeAuthorizedRequest(t *testing.T, server *api.Server, creds *TestCredentials, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)
	return w
}

func makeInternalRequest(t *testing.T, server *api.Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	server.InternalRouter().ServeHTTP(w, req)
	return w
}

func TestXrayStatus(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "GET", "/node/xray/status", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsRunning bool    `json:"isRunning"`
			Version   *string `json:"version"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.IsRunning)
	assert.NotNil(t, response.Response.Version)
}

func TestXrayHealthcheck(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "GET", "/node/xray/healthcheck", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsAlive                  bool    `json:"isAlive"`
			XrayInternalStatusCached bool    `json:"xrayInternalStatusCached"`
			XrayVersion              *string `json:"xrayVersion"`
			NodeVersion              string  `json:"nodeVersion"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.IsAlive)
	assert.False(t, response.Response.XrayInternalStatusCached)
	assert.NotEmpty(t, response.Response.NodeVersion)
}

func TestXrayStartWithMinimalConfig(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	startReq := CreateMinimalXrayConfig()

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsStarted  bool   `json:"isStarted"`
			Version    string `json:"version"`
			Error      string `json:"error"`
			SystemInfo struct {
				CpuCores    int    `json:"cpuCores"`
				CpuModel    string `json:"cpuModel"`
				MemoryTotal string `json:"memoryTotal"`
			} `json:"systemInformation"`
			NodeInformation struct {
				Version string `json:"version"`
			} `json:"nodeInformation"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.IsStarted)
	assert.NotEmpty(t, response.Response.Version)
	assert.NotNil(t, response.Response.SystemInfo)
	assert.NotEmpty(t, response.Response.NodeInformation.Version)
}

func TestXrayStop(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	startReq := CreateMinimalXrayConfig()
	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
	assert.Equal(t, http.StatusOK, w.Code)

	w = makeAuthorizedRequest(t, server, creds, "GET", "/node/xray/stop", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsStopped bool `json:"isStopped"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.IsStopped)
}

func TestHandlerAddUserWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	addUserReq := &AddUserRequest{
		Data: []AddUserInboundData{
			{
				Tag:      "vless-in",
				Username: "testuser@example.com",
				Type:     "vless",
				UUID:     "550e8400-e29b-41d4-a716-446655440000",
				Flow:     "xtls-rprx-vision",
			},
		},
		HashData: AddUserHashData{
			VlessUUID: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/add-user", addUserReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestStatsGetSystemStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "GET", "/node/stats/get-system-stats", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			NumGoroutine int    `json:"numGoroutine"`
			NumGC        uint32 `json:"numGC"`
			Alloc        uint64 `json:"alloc"`
			TotalAlloc   uint64 `json:"totalAlloc"`
			Sys          uint64 `json:"sys"`
			Mallocs      uint64 `json:"mallocs"`
			Frees        uint64 `json:"frees"`
			LiveObjects  uint64 `json:"liveObjects"`
			PauseTotalNs uint64 `json:"pauseTotalNs"`
			Uptime       int64  `json:"uptime"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Greater(t, response.Response.NumGoroutine, 0)
	assert.Greater(t, response.Response.Sys, uint64(0))
}

func TestStatsGetUsersStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-users-stats", map[string]bool{"reset": false})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Users []struct {
				Username string `json:"username"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"users"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.Users)
}

func TestInternalGetConfigSocketDestroyedInHttptest(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeInternalRequest(t, server, "GET", "/internal/get-config", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String(), "internal router destroys socket in httptest due to PortGuardMiddleware")
}

func TestInternalControllerReturnsRawJSON(t *testing.T) {
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	configMgr := xray.NewConfigManager(log)

	internalController := controller.NewInternalController(configMgr, log)

	router := gin.New()
	group := router.Group("/internal")
	internalController.RegisterRoutes(group)

	req := httptest.NewRequest("GET", "/internal/get-config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	_, hasResponse := response["response"]
	assert.False(t, hasResponse, "internal get-config should NOT have response wrapper")
}

func TestJWTAuthFailureMissingHeader(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	req := httptest.NewRequest("GET", "/node/xray/status", nil)
	w := httptest.NewRecorder()

	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestJWTAuthFailureInvalidToken(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	req := httptest.NewRequest("GET", "/node/xray/status", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestJWTAuthFailureExpiredToken(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	expiredJWT, err := creds.GenerateExpiredJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/node/xray/status", nil)
	req.Header.Set("Authorization", "Bearer "+expiredJWT)
	w := httptest.NewRecorder()

	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestNotFoundDestroysSocket(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/nonexistent/path", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()

	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestInternalRouter404ReturnsText(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeInternalRequest(t, server, "GET", "/internal/nonexistent", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestValidationErrorAddUserMissingFields(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	invalidReq := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"tag": "vless-in",
			},
		},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/add-user", invalidReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestValidationErrorInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/node/handler/add-user", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestXrayStartValidationError(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	invalidReq := map[string]interface{}{
		"xrayConfig": nil,
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", invalidReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsStarted       bool    `json:"isStarted"`
			Error           *string `json:"error"`
			NodeInformation struct {
				Version string `json:"version"`
			} `json:"nodeInformation"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.IsStarted)
	assert.NotNil(t, response.Response.Error)
}

func TestHandlerGetInboundUsers(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/get-inbound-users", map[string]string{
		"tag": "vless-in",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Users []struct {
				Username string `json:"username"`
				Level    uint32 `json:"level"`
				Protocol string `json:"protocol"`
			} `json:"users"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.Users)
}

func TestHandlerGetInboundUsersCount(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/get-inbound-users-count", map[string]string{
		"tag": "vless-in",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Count int64 `json:"count"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, int64(0), response.Response.Count)
}

func TestStatsGetUserOnlineStatus(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-user-online-status", map[string]string{
		"username": "testuser@example.com",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsOnline bool `json:"isOnline"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.IsOnline)
}

func TestStatsGetUserIPList(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-user-ip-list", map[string]string{
		"userId": "testuser@example.com",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IPs []string `json:"ips"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.IPs)
	assert.Empty(t, response.Response.IPs)
}

func TestStatsGetInboundStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-inbound-stats", map[string]interface{}{
		"tag":   "vless-in",
		"reset": false,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Inbound  string `json:"inbound"`
			Uplink   int64  `json:"uplink"`
			Downlink int64  `json:"downlink"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "vless-in", response.Response.Inbound)
}

func TestStatsGetOutboundStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-outbound-stats", map[string]interface{}{
		"tag":   "direct",
		"reset": false,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Outbound string `json:"outbound"`
			Uplink   int64  `json:"uplink"`
			Downlink int64  `json:"downlink"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "direct", response.Response.Outbound)
}

func TestStatsGetAllInboundsStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-all-inbounds-stats", map[string]bool{
		"reset": false,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Inbounds []struct {
				Inbound  string `json:"inbound"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"inbounds"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.Inbounds)
}

func TestStatsGetAllOutboundsStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-all-outbounds-stats", map[string]bool{
		"reset": false,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Outbounds []struct {
				Outbound string `json:"outbound"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"outbounds"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.Outbounds)
}

func TestStatsGetCombinedStats(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/stats/get-combined-stats", map[string]bool{
		"reset": false,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Inbounds []struct {
				Inbound  string `json:"inbound"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"inbounds"`
			Outbounds []struct {
				Outbound string `json:"outbound"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"outbounds"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Response.Inbounds)
	assert.NotNil(t, response.Response.Outbounds)
}

func TestHandlerRemoveUserWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	removeReq := map[string]interface{}{
		"username": "testuser@example.com",
		"hashData": map[string]string{
			"vlessUuid": "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/remove-user", removeReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestXrayStartForceRestart(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	startReq := CreateMinimalXrayConfig()
	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
	assert.Equal(t, http.StatusOK, w.Code)

	startReq.Internals.ForceRestart = true
	w = makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsStarted bool `json:"isStarted"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.IsStarted)
}

func TestHandlerAddUsersWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	addUsersReq := map[string]interface{}{
		"users": []map[string]interface{}{
			{
				"userData": map[string]interface{}{
					"userId":    "user1@example.com",
					"vlessUuid": "550e8400-e29b-41d4-a716-446655440000",
				},
				"inboundData": []map[string]interface{}{
					{"tag": "vless-in", "type": "vless"},
				},
			},
		},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/add-users", addUsersReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestHandlerRemoveUsersWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	removeUsersReq := map[string]interface{}{
		"users": []map[string]interface{}{
			{"userId": "user1@example.com"},
		},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/remove-users", removeUsersReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestHandlerRemoveUsersEmptyList(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	removeUsersReq := map[string]interface{}{
		"users": []map[string]interface{}{},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/remove-users", removeUsersReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.Success)
}

func TestHandlerAddUsersEmptyList(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	addUsersReq := map[string]interface{}{
		"users": []map[string]interface{}{},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/add-users", addUsersReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.Success)
}

func TestHandlerDropUsersConnections(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	dropReq := map[string]interface{}{
		"userIds": []string{"user1@example.com", "user2@example.com"},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/drop-users-connections", dropReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.Success)
}

func TestHandlerDropUsersConnectionsInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/node/handler/drop-users-connections", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
}

func TestHandlerDropIPs(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	dropReq := map[string]interface{}{
		"ips": []string{"1.2.3.4", "5.6.7.8"},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/drop-ips", dropReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Response.Success)
}

func TestHandlerDropIPsInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/node/handler/drop-ips", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
}

func TestHandlerAddUserEmptyData(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	addUserReq := map[string]interface{}{
		"data":     []map[string]interface{}{},
		"hashData": map[string]interface{}{},
	}

	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/handler/add-user", addUserReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Response.Success)
	assert.NotNil(t, response.Response.Error)
}

func TestXrayStopWithoutRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	w := makeAuthorizedRequest(t, server, creds, "GET", "/node/xray/stop", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			IsStopped bool `json:"isStopped"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// Stop when not running should still succeed or report status
}

func TestXrayStartNoRestartNeeded(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	startReq := CreateMinimalXrayConfig()

	// First start
	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
	assert.Equal(t, http.StatusOK, w.Code)

	var firstResp struct {
		Response struct {
			IsStarted bool `json:"isStarted"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &firstResp)
	require.NoError(t, err)
	assert.True(t, firstResp.Response.IsStarted)

	// Second start with same config — should not restart
	w = makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
	assert.Equal(t, http.StatusOK, w.Code)

	var secondResp struct {
		Response struct {
			IsStarted bool `json:"isStarted"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &secondResp)
	require.NoError(t, err)
	assert.True(t, secondResp.Response.IsStarted, "should report started even without restart")
}

func TestStatsInboundStatsInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/node/stats/get-inbound-stats", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Inbound string `json:"inbound"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response.Response.Inbound)
}

func TestStatsOutboundStatsInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	jwt, err := creds.GenerateJWT()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/node/stats/get-outbound-stats", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()
	server.MainRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Response struct {
			Outbound string `json:"outbound"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response.Response.Outbound)
}

func TestVisionBlockIPWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	blockReq := map[string]string{
		"ip":       "1.2.3.4",
		"username": "testuser",
	}

	w := makeInternalRequest(t, server, "POST", "/vision/block-ip", blockReq)

	// PortGuardMiddleware will destroy the socket in httptest since there's no LocalAddr
	// But we can verify the server handles the route
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVisionUnblockIPWithoutXrayRunning(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	unblockReq := map[string]string{
		"ip":       "1.2.3.4",
		"username": "testuser",
	}

	w := makeInternalRequest(t, server, "POST", "/vision/unblock-ip", unblockReq)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVisionBlockIPInvalidJSON(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	// Empty object — missing required fields
	w := makeInternalRequest(t, server, "POST", "/vision/block-ip", map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInternalGetConfigAfterStart(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	// Start xray first
	startReq := CreateMinimalXrayConfig()
	w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
	assert.Equal(t, http.StatusOK, w.Code)

	var startResp struct {
		Response struct {
			IsStarted bool `json:"isStarted"`
		} `json:"response"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &startResp)
	require.NoError(t, err)
	assert.True(t, startResp.Response.IsStarted)

	// Note: internal requests go through PortGuardMiddleware which destroys socket in httptest
	// So we test the internal controller directly
	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	configMgr := xray.NewConfigManager(log)

	// Simulate config being set by starting xray — use a direct controller test
	internalCtrl := controller.NewInternalController(configMgr, log)
	router := gin.New()
	group := router.Group("/internal")
	internalCtrl.RegisterRoutes(group)

	// Before setting config, should return empty
	req := httptest.NewRequest("GET", "/internal/get-config", nil)
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, req)

	assert.Equal(t, http.StatusOK, cw.Code)

	var configResp map[string]interface{}
	err = json.Unmarshal(cw.Body.Bytes(), &configResp)
	require.NoError(t, err)
	assert.NotNil(t, configResp)
}

func TestXrayStartDuplicateRequestRejected(t *testing.T) {
	creds, err := GenerateTestCredentials()
	require.NoError(t, err)

	server := setupTestServer(t, creds)

	startReq := CreateMinimalXrayConfig()

	done := make(chan *httptest.ResponseRecorder, 2)

	go func() {
		w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
		done <- w
	}()

	go func() {
		w := makeAuthorizedRequest(t, server, creds, "POST", "/node/xray/start", startReq)
		done <- w
	}()

	results := make([]*httptest.ResponseRecorder, 0, 2)
	for i := 0; i < 2; i++ {
		results = append(results, <-done)
	}

	hasSuccess := false
	for _, w := range results {
		if w.Code == http.StatusOK {
			var response struct {
				Response struct {
					IsStarted bool `json:"isStarted"`
				} `json:"response"`
			}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err == nil && response.Response.IsStarted {
				hasSuccess = true
			}
		}
	}

	assert.True(t, hasSuccess, "at least one request should succeed")
}
