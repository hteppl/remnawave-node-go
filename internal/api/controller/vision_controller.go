package controller

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

var (
	logFailedToParseBlockIPRequest   = "Failed to parse block-ip request"
	logFailedToParseUnblockIPRequest = "Failed to parse unblock-ip request"
	logFailedToAddRoutingRule        = "Failed to add routing rule"
	logFailedToRemoveRoutingRule     = "Failed to remove routing rule"
	logIPBlocked                     = "IP blocked"
	logIPUnblocked                   = "IP unblocked"
)

type BlockIPRequest struct {
	IP       string `json:"ip" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type BlockIPResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
}

type VisionController struct {
	core       *xray.Core
	logger     *logger.Logger
	blockedIPs map[string]string
	mu         sync.RWMutex
}

func NewVisionController(core *xray.Core, log *logger.Logger) *VisionController {
	return &VisionController{
		core:       core,
		logger:     log,
		blockedIPs: make(map[string]string),
	}
}

func (c *VisionController) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/block-ip", c.handleBlockIP)
	group.POST("/unblock-ip", c.handleUnblockIP)
}

func (c *VisionController) getIPHash(ip string) string {
	// Match object-hash npm package format: "string:<length>:<value>"
	data := fmt.Sprintf("string:%d:%s", len(ip), ip)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (c *VisionController) handleBlockIP(ctx *gin.Context) {
	var req BlockIPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error(logFailedToParseBlockIPRequest)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	ruleTag := c.getIPHash(req.IP)

	if err := c.core.AddRoutingRule(ruleTag, req.IP, "BLOCK"); err != nil {
		c.logger.WithError(err).WithField("ip", req.IP).Error(logFailedToAddRoutingRule)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	c.mu.Lock()
	c.blockedIPs[ruleTag] = req.IP
	c.mu.Unlock()

	c.logger.WithField("ip", req.IP).WithField("ruleTag", ruleTag).Info(logIPBlocked)

	ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
		Success: true,
		Error:   nil,
	}))
}

func (c *VisionController) handleUnblockIP(ctx *gin.Context) {
	var req BlockIPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error(logFailedToParseUnblockIPRequest)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	ruleTag := c.getIPHash(req.IP)

	if err := c.core.RemoveRoutingRule(ruleTag); err != nil {
		c.logger.WithError(err).WithField("ip", req.IP).Error(logFailedToRemoveRoutingRule)
		errMsg := err.Error()
		ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	c.mu.Lock()
	delete(c.blockedIPs, ruleTag)
	c.mu.Unlock()

	c.logger.WithField("ip", req.IP).WithField("ruleTag", ruleTag).Info(logIPUnblocked)

	ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
		Success: true,
		Error:   nil,
	}))
}

func (c *VisionController) GetBlockedIPs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ips := make([]string, 0, len(c.blockedIPs))
	for _, ip := range c.blockedIPs {
		ips = append(ips, ip)
	}
	return ips
}

func (c *VisionController) IsBlocked(ip string) bool {
	ruleTag := c.getIPHash(ip)
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, blocked := c.blockedIPs[ruleTag]
	return blocked
}
