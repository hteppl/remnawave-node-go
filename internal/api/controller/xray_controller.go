package controller

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/utils"
	"github.com/hteppl/remnawave-node-go/internal/version"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type successResponse struct {
	Response interface{} `json:"response"`
}

func wrapResponse(data interface{}) successResponse {
	return successResponse{Response: data}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const (
	APIPort = 61012
)

var (
	msgRequestAlreadyInProgress  = "Request already in progress"
	msgUnsupportedVersion        = "Unsupported Remnawave version. Please, upgrade Remnawave to version v2.3.x or higher"
	logStartAlreadyInProgress    = "Start request already in progress, rejecting duplicate"
	logFailedToParseStartRequest = "Failed to parse start request"
	logRestartRequired           = "Restart required - proceeding with xray core restart"
	logForceRestartRequested     = "Force restart requested"
	logFailedToExtractUsers      = "Failed to extract users from config"
	logFailedToMarshalConfig     = "Failed to marshal xray config"
	logFailedToStartXray         = "Failed to start xray core"
	logXrayStartedSuccessfully   = "Xray core started successfully"
	logStopRequested             = "Remnawave requested to stop Xray."
	logFailedToStopXray          = "Failed to stop xray core"
	logXrayStoppedSuccessfully   = "Xray core stopped successfully"
)

type StartRequest struct {
	XrayConfig map[string]interface{} `json:"xrayConfig" binding:"required"`
	Internals  xray.Internals         `json:"internals" binding:"required"`
}

type NodeInformation struct {
	Version string `json:"version"`
}

type SystemInfo struct {
	CpuCores    int    `json:"cpuCores"`
	CpuModel    string `json:"cpuModel"`
	MemoryTotal string `json:"memoryTotal"`
}

type StartResponse struct {
	IsStarted       bool            `json:"isStarted"`
	Version         *string         `json:"version"`
	Error           *string         `json:"error"`
	SystemInfo      *SystemInfo     `json:"systemInformation"`
	NodeInformation NodeInformation `json:"nodeInformation"`
}

type StopResponse struct {
	IsStopped bool `json:"isStopped"`
}

type StatusResponse struct {
	IsRunning bool    `json:"isRunning"`
	Version   *string `json:"version"`
}

type HealthcheckResponse struct {
	IsAlive                  bool    `json:"isAlive"`
	XrayInternalStatusCached bool    `json:"xrayInternalStatusCached"`
	XrayVersion              *string `json:"xrayVersion"`
	NodeVersion              string  `json:"nodeVersion"`
}

type XrayController struct {
	core          *xray.Core
	configManager *xray.ConfigManager
	logger        *logger.Logger
	startMu       sync.Mutex
	isProcessing  atomic.Bool
}

func NewXrayController(core *xray.Core, configManager *xray.ConfigManager, log *logger.Logger) *XrayController {
	return &XrayController{
		core:          core,
		configManager: configManager,
		logger:        log,
	}
}

func (c *XrayController) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/start", c.handleStart)
	group.GET("/stop", c.handleStop)
	group.GET("/status", c.handleStatus)
	group.GET("/healthcheck", c.handleHealthcheck)
}

func (c *XrayController) handleStart(ctx *gin.Context) {
	xrayVer := c.core.GetVersion()
	nodeInfo := NodeInformation{Version: version.Version}

	if !c.isProcessing.CompareAndSwap(false, true) {
		c.logger.Warn(logStartAlreadyInProgress)
		ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
			IsStarted:       false,
			Version:         strPtr(xrayVer),
			Error:           &msgRequestAlreadyInProgress,
			NodeInformation: nodeInfo,
		}))
		return
	}
	defer c.isProcessing.Store(false)

	c.startMu.Lock()
	defer c.startMu.Unlock()

	var req StartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error(logFailedToParseStartRequest)
		ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
			IsStarted:       false,
			Error:           &msgUnsupportedVersion,
			NodeInformation: nodeInfo,
		}))
		return
	}

	hashes := req.Internals.Hashes
	forceRestart := req.Internals.ForceRestart

	if c.core.IsRunning() && !forceRestart {
		needRestart := c.configManager.IsNeedRestartCore(hashes)
		if !needRestart {
			xrayVer = c.core.GetVersion()
			sysInfo := getSystemInfo()
			ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
				IsStarted:       true,
				Version:         &xrayVer,
				SystemInfo:      &sysInfo,
				NodeInformation: nodeInfo,
			}))
			return
		}
		c.logger.Info(logRestartRequired)
	}

	if forceRestart {
		c.logger.Warn(logForceRestartRequested)
	}

	config := generateAPIConfig(req.XrayConfig)

	if err := c.configManager.ExtractUsersFromConfig(hashes, config); err != nil {
		c.logger.WithError(err).Error(logFailedToExtractUsers)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
			IsStarted:       false,
			Error:           &errMsg,
			NodeInformation: nodeInfo,
		}))
		return
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		c.logger.WithError(err).Error(logFailedToMarshalConfig)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
			IsStarted:       false,
			Error:           &errMsg,
			NodeInformation: nodeInfo,
		}))
		return
	}

	if err := c.core.Start(configJSON); err != nil {
		c.logger.WithError(err).Error(logFailedToStartXray)
		errMsg := err.Error()
		sysInfo := getSystemInfo()
		ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
			IsStarted:       false,
			Error:           &errMsg,
			SystemInfo:      &sysInfo,
			NodeInformation: nodeInfo,
		}))
		return
	}

	xrayVer = c.core.GetVersion()
	sysInfo := getSystemInfo()

	c.logger.WithField("version", xrayVer).Info(logXrayStartedSuccessfully)

	ctx.JSON(http.StatusOK, wrapResponse(StartResponse{
		IsStarted:       true,
		Version:         &xrayVer,
		SystemInfo:      &sysInfo,
		NodeInformation: nodeInfo,
	}))
}

func (c *XrayController) handleStop(ctx *gin.Context) {
	c.logger.Info(logStopRequested)

	c.startMu.Lock()
	defer c.startMu.Unlock()

	if err := c.core.Stop(); err != nil {
		c.logger.WithError(err).Error(logFailedToStopXray)
		ctx.JSON(http.StatusOK, wrapResponse(StopResponse{
			IsStopped: false,
		}))
		return
	}

	c.configManager.Cleanup()

	c.logger.Info(logXrayStoppedSuccessfully)

	ctx.JSON(http.StatusOK, wrapResponse(StopResponse{
		IsStopped: true,
	}))
}

func (c *XrayController) handleStatus(ctx *gin.Context) {
	isRunning := c.core.IsRunning()
	var version *string
	if isRunning {
		v := c.core.GetVersion()
		version = &v
	}

	ctx.JSON(http.StatusOK, wrapResponse(StatusResponse{
		IsRunning: isRunning,
		Version:   version,
	}))
}

func (c *XrayController) handleHealthcheck(ctx *gin.Context) {
	isRunning := c.core.IsRunning()
	var xrayVersion *string
	if isRunning {
		v := c.core.GetVersion()
		xrayVersion = &v
	}

	ctx.JSON(http.StatusOK, wrapResponse(HealthcheckResponse{
		IsAlive:                  true,
		XrayInternalStatusCached: isRunning,
		XrayVersion:              xrayVersion,
		NodeVersion:              version.Version,
	}))
}

func getSystemInfo() SystemInfo {
	return SystemInfo{
		CpuCores:    utils.GetCPUCores(),
		CpuModel:    utils.GetCPUModel(),
		MemoryTotal: utils.GetTotalMemory(),
	}
}

func generateAPIConfig(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		result[k] = v
	}

	apiInbound := map[string]interface{}{
		"tag":      "api",
		"port":     APIPort,
		"listen":   "127.0.0.1",
		"protocol": "dokodemo-door",
		"settings": map[string]interface{}{
			"address": "127.0.0.1",
		},
	}

	inbounds, ok := result["inbounds"].([]interface{})
	if !ok {
		inbounds = []interface{}{}
	}

	hasAPIInbound := false
	for _, inbound := range inbounds {
		if ib, ok := inbound.(map[string]interface{}); ok {
			if tag, ok := ib["tag"].(string); ok && tag == "api" {
				hasAPIInbound = true
				break
			}
		}
	}

	if !hasAPIInbound {
		inbounds = append(inbounds, apiInbound)
		result["inbounds"] = inbounds
	}

	routing, ok := result["routing"].(map[string]interface{})
	if !ok {
		routing = map[string]interface{}{}
	}

	rules, ok := routing["rules"].([]interface{})
	if !ok {
		rules = []interface{}{}
	}

	hasAPIRule := false
	for _, rule := range rules {
		if r, ok := rule.(map[string]interface{}); ok {
			if outboundTag, ok := r["outboundTag"].(string); ok && outboundTag == "api" {
				hasAPIRule = true
				break
			}
		}
	}

	if !hasAPIRule {
		apiRule := map[string]interface{}{
			"type":        "field",
			"outboundTag": "api",
			"inboundTag":  []interface{}{"api"},
		}
		rules = append([]interface{}{apiRule}, rules...)
		routing["rules"] = rules
		result["routing"] = routing
	}

	if _, ok := result["api"]; !ok {
		result["api"] = map[string]interface{}{
			"services": []interface{}{"HandlerService", "LoggerService", "StatsService", "RoutingService"},
			"tag":      "api",
		}
	} else {
		api, _ := result["api"].(map[string]interface{})
		if api != nil {
			services, _ := api["services"].([]interface{})
			hasRoutingService := false
			for _, s := range services {
				if str, ok := s.(string); ok && str == "RoutingService" {
					hasRoutingService = true
					break
				}
			}
			if !hasRoutingService {
				api["services"] = append(services, "RoutingService")
			}
		}
	}

	if _, ok := result["stats"]; !ok {
		result["stats"] = map[string]interface{}{}
	}

	outbounds, _ := result["outbounds"].([]interface{})
	hasBlockOutbound := false
	for _, ob := range outbounds {
		if outbound, ok := ob.(map[string]interface{}); ok {
			if tag, ok := outbound["tag"].(string); ok && tag == "BLOCK" {
				hasBlockOutbound = true
				break
			}
		}
	}
	if !hasBlockOutbound {
		blockOutbound := map[string]interface{}{
			"tag":      "BLOCK",
			"protocol": "blackhole",
			"settings": map[string]interface{}{
				"response": map[string]interface{}{
					"type": "http",
				},
			},
		}
		outbounds = append(outbounds, blockOutbound)
		result["outbounds"] = outbounds
	}

	existingPolicy, _ := result["policy"].(map[string]interface{})
	if existingPolicy == nil {
		existingPolicy = map[string]interface{}{}
	}

	existingLevels, _ := existingPolicy["levels"].(map[string]interface{})
	if existingLevels == nil {
		existingLevels = map[string]interface{}{}
	}

	existingLevel0, _ := existingLevels["0"].(map[string]interface{})
	if existingLevel0 == nil {
		existingLevel0 = map[string]interface{}{}
	}

	existingLevel0["statsUserUplink"] = true
	existingLevel0["statsUserDownlink"] = true
	existingLevel0["statsUserOnline"] = false

	existingLevels["0"] = existingLevel0
	existingPolicy["levels"] = existingLevels

	existingPolicy["system"] = map[string]interface{}{
		"statsInboundUplink":    true,
		"statsInboundDownlink":  true,
		"statsOutboundUplink":   true,
		"statsOutboundDownlink": true,
	}

	result["policy"] = existingPolicy

	return result
}
