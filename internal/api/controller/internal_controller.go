package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type InternalController struct {
	configManager *xray.ConfigManager
	logger        *logger.Logger
}

func NewInternalController(configManager *xray.ConfigManager, log *logger.Logger) *InternalController {
	return &InternalController{
		configManager: configManager,
		logger:        log,
	}
}

func (c *InternalController) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/get-config", c.handleGetConfig)
}

func (c *InternalController) handleGetConfig(ctx *gin.Context) {
	config := c.configManager.GetXrayConfig()
	ctx.JSON(http.StatusOK, config)
}
