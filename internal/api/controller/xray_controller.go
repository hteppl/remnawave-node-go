package controller

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/version"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

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
	v := c.core.GetVersion()

	ctx.JSON(http.StatusOK, wrapResponse(StatusResponse{
		IsRunning: isRunning,
		Version:   &v,
	}))
}

func (c *XrayController) handleHealthcheck(ctx *gin.Context) {
	isRunning := c.core.IsRunning()
	v := c.core.GetVersion()

	ctx.JSON(http.StatusOK, wrapResponse(HealthcheckResponse{
		IsAlive:                  true,
		XrayInternalStatusCached: isRunning,
		XrayVersion:              &v,
		NodeVersion:              version.Version,
	}))
}
