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

func startXrayWithConfig(t *testing.T, xrayConfig map[string]interface{}) (map[string]interface{}, *httptest.ResponseRecorder) {
	t.Helper()

	log := logger.New(logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})
	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	xrayCtrl := controller.NewXrayController(core, configMgr, log)
	internalCtrl := controller.NewInternalController(configMgr, log)

	router := gin.New()
	xrayGroup := router.Group("/node/xray")
	xrayCtrl.RegisterRoutes(xrayGroup)
	internalGroup := router.Group("/internal")
	internalCtrl.RegisterRoutes(internalGroup)

	startReq := map[string]interface{}{
		"xrayConfig": xrayConfig,
		"internals": map[string]interface{}{
			"forceRestart": false,
			"hashes": map[string]interface{}{
				"emptyConfig": "abc123",
				"inbounds":    []interface{}{},
			},
		},
	}

	req := httptest.NewRequest("POST", "/node/xray/start", jsonBody(t, startReq))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Now get the config from internal endpoint
	configReq := httptest.NewRequest("GET", "/internal/get-config", nil)
	configW := httptest.NewRecorder()
	router.ServeHTTP(configW, configReq)

	var config map[string]interface{}
	if configW.Body.Len() > 0 {
		_ = json.Unmarshal(configW.Body.Bytes(), &config)
	}

	return config, w
}

func TestGenerateAPIConfig_AddsAPIInbound(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"port":     10000,
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients":    []interface{}{},
					"decryption": "none",
				},
				"streamSettings": map[string]interface{}{
					"network": "tcp",
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"tag":      "direct",
				"protocol": "freedom",
			},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)

	assert.Equal(t, http.StatusOK, w.Code)

	var startResp struct {
		Response struct {
			IsStarted bool `json:"isStarted"`
		} `json:"response"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &startResp)
	require.NoError(t, err)
	assert.True(t, startResp.Response.IsStarted)

	require.NotNil(t, storedConfig)

	// Check that API inbound was added
	inbounds, ok := storedConfig["inbounds"].([]interface{})
	require.True(t, ok)

	hasAPIInbound := false
	for _, ib := range inbounds {
		ibMap, ok := ib.(map[string]interface{})
		if ok {
			if tag, ok := ibMap["tag"].(string); ok && tag == "REMNAWAVE_API_INBOUND" {
				hasAPIInbound = true
				assert.Equal(t, "dokodemo-door", ibMap["protocol"])
				break
			}
		}
	}
	assert.True(t, hasAPIInbound, "should have REMNAWAVE_API_INBOUND inbound")
}

func TestGenerateAPIConfig_AddsAPIRoutingRule(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	routing, ok := storedConfig["routing"].(map[string]interface{})
	require.True(t, ok)
	rules, ok := routing["rules"].([]interface{})
	require.True(t, ok)

	hasAPIRule := false
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if ok {
			if outTag, ok := ruleMap["outboundTag"].(string); ok && outTag == "REMNAWAVE_API" {
				hasAPIRule = true
				break
			}
		}
	}
	assert.True(t, hasAPIRule, "should have REMNAWAVE_API routing rule")
}

func TestGenerateAPIConfig_AddsStats(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	_, hasStats := storedConfig["stats"]
	assert.True(t, hasStats, "should have stats section")
}

func TestGenerateAPIConfig_AddsBLOCKOutbound(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	outbounds, ok := storedConfig["outbounds"].([]interface{})
	require.True(t, ok)

	hasBlock := false
	for _, ob := range outbounds {
		obMap, ok := ob.(map[string]interface{})
		if ok {
			if tag, ok := obMap["tag"].(string); ok && tag == "BLOCK" {
				hasBlock = true
				assert.Equal(t, "blackhole", obMap["protocol"])
				break
			}
		}
	}
	assert.True(t, hasBlock, "should have BLOCK outbound")
}

func TestGenerateAPIConfig_AddsPolicy(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	policy, ok := storedConfig["policy"].(map[string]interface{})
	require.True(t, ok, "should have policy section")

	system, ok := policy["system"].(map[string]interface{})
	require.True(t, ok, "should have system policy")
	assert.Equal(t, true, system["statsInboundUplink"])
	assert.Equal(t, true, system["statsInboundDownlink"])
	assert.Equal(t, true, system["statsOutboundUplink"])
	assert.Equal(t, true, system["statsOutboundDownlink"])
}

func TestGenerateAPIConfig_SkipsDuplicateAPIInbound(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "REMNAWAVE_API_INBOUND", "port": 99999, "protocol": "dokodemo-door",
				"settings": map[string]interface{}{"address": "127.0.0.1"},
			},
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	inbounds, ok := storedConfig["inbounds"].([]interface{})
	require.True(t, ok)

	apiCount := 0
	for _, ib := range inbounds {
		ibMap, ok := ib.(map[string]interface{})
		if ok {
			if tag, ok := ibMap["tag"].(string); ok && tag == "REMNAWAVE_API_INBOUND" {
				apiCount++
			}
		}
	}
	assert.Equal(t, 1, apiCount, "should not duplicate REMNAWAVE_API_INBOUND inbound")
}

func TestGenerateAPIConfig_AddsRoutingServiceToExistingAPI(t *testing.T) {
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
		"api": map[string]interface{}{
			"services": []interface{}{"HandlerService", "StatsService"},
			"tag":      "api",
		},
	}

	storedConfig, w := startXrayWithConfig(t, config)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storedConfig)

	apiSection, ok := storedConfig["api"].(map[string]interface{})
	require.True(t, ok)

	services, ok := apiSection["services"].([]interface{})
	require.True(t, ok)

	hasRouting := false
	for _, s := range services {
		if str, ok := s.(string); ok && str == "RoutingService" {
			hasRouting = true
			break
		}
	}
	assert.True(t, hasRouting, "should add RoutingService to existing api section")
}

func TestGenerateAPIConfig_DoesNotMutateOriginal(t *testing.T) {
	original := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in", "port": 10000, "protocol": "vless",
				"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none"},
				"streamSettings": map[string]interface{}{"network": "tcp"},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "direct", "protocol": "freedom"},
		},
	}

	originalJSON, err := json.Marshal(original)
	require.NoError(t, err)

	startXrayWithConfig(t, original)

	afterJSON, err := json.Marshal(original)
	require.NoError(t, err)

	assert.JSONEq(t, string(originalJSON), string(afterJSON), "generateAPIConfig should not mutate original map")
}
