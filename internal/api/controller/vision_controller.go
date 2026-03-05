package controller

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type BlockIPRequest struct {
	IP string `json:"ip" binding:"required"`
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
	hash := md5.Sum([]byte(ip))
	return hex.EncodeToString(hash[:])
}

func (c *VisionController) handleBlockIP(ctx *gin.Context) {
	var req BlockIPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error("Failed to parse block-ip request")
		errMsg := "invalid request body: " + err.Error()
		ctx.JSON(http.StatusBadRequest, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	if net.ParseIP(req.IP) == nil {
		errMsg := "invalid IP address format"
		ctx.JSON(http.StatusBadRequest, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	ruleTag := c.getIPHash(req.IP)

	c.mu.Lock()
	_, alreadyBlocked := c.blockedIPs[ruleTag]
	if alreadyBlocked {
		c.mu.Unlock()
		ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
			Success: true,
			Error:   nil,
		}))
		return
	}
	c.blockedIPs[ruleTag] = req.IP
	c.mu.Unlock()

	if err := c.core.AddRoutingRule(ruleTag, req.IP, "BLOCK"); err != nil {
		c.logger.WithError(err).WithField("ip", req.IP).Error("Failed to add routing rule")

		c.mu.Lock()
		delete(c.blockedIPs, ruleTag)
		c.mu.Unlock()

		errMsg := "failed to block IP: " + err.Error()
		ctx.JSON(http.StatusInternalServerError, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	c.logger.WithField("ip", req.IP).WithField("ruleTag", ruleTag).Info("IP blocked")

	ctx.JSON(http.StatusOK, wrapResponse(BlockIPResponse{
		Success: true,
		Error:   nil,
	}))
}

func (c *VisionController) handleUnblockIP(ctx *gin.Context) {
	var req BlockIPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error("Failed to parse unblock-ip request")
		errMsg := "invalid request body: " + err.Error()
		ctx.JSON(http.StatusBadRequest, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	if net.ParseIP(req.IP) == nil {
		errMsg := "invalid IP address format"
		ctx.JSON(http.StatusBadRequest, wrapResponse(BlockIPResponse{
			Success: false,
			Error:   &errMsg,
		}))
		return
	}

	ruleTag := c.getIPHash(req.IP)

	c.mu.Lock()
	_, wasBlocked := c.blockedIPs[ruleTag]
	delete(c.blockedIPs, ruleTag)
	c.mu.Unlock()

	if wasBlocked {
		if err := c.core.RemoveRoutingRule(ruleTag); err != nil {
			c.logger.WithError(err).WithField("ip", req.IP).Warn("Failed to remove routing rule")
		}
	}

	c.logger.WithField("ip", req.IP).WithField("ruleTag", ruleTag).Info("IP unblocked")

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
