package controller

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type StatsController struct {
	core      *xray.Core
	logger    *logger.Logger
	startTime time.Time
}

func NewStatsController(core *xray.Core, log *logger.Logger) *StatsController {
	return &StatsController{
		core:      core,
		logger:    log,
		startTime: time.Now(),
	}
}

func (c *StatsController) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/get-system-stats", c.handleGetSystemStats)
	group.POST("/get-users-stats", c.handleGetUsersStats)
	group.POST("/get-user-online-status", c.handleGetUserOnlineStatus)
	group.POST("/get-user-ip-list", c.handleGetUserIPList)
	group.POST("/get-inbound-stats", c.handleGetInboundStats)
	group.POST("/get-outbound-stats", c.handleGetOutboundStats)
	group.POST("/get-all-inbounds-stats", c.handleGetAllInboundsStats)
	group.POST("/get-all-outbounds-stats", c.handleGetAllOutboundsStats)
	group.POST("/get-combined-stats", c.handleGetCombinedStats)
}

func (c *StatsController) handleGetSystemStats(ctx *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := int64(time.Since(c.startTime).Seconds())

	ctx.JSON(http.StatusOK, wrapResponse(SystemStatsResponse{
		NumGoroutine: runtime.NumGoroutine(),
		NumGC:        memStats.NumGC,
		Alloc:        memStats.Alloc,
		TotalAlloc:   memStats.TotalAlloc,
		Sys:          memStats.Sys,
		Mallocs:      memStats.Mallocs,
		Frees:        memStats.Frees,
		LiveObjects:  memStats.Mallocs - memStats.Frees,
		PauseTotalNs: memStats.PauseTotalNs,
		Uptime:       uptime,
	}))
}

func (c *StatsController) handleGetUsersStats(ctx *gin.Context) {
	var req ResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Reset = false
	}

	stm := c.getConcreteStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(UsersStatsResponse{
			Users: []UserStats{},
		}))
		return
	}

	userTraffic := c.collectUserStats(stm, req.Reset)

	users := make([]UserStats, 0, len(userTraffic))
	for _, userStats := range userTraffic {
		if userStats.Uplink != 0 || userStats.Downlink != 0 {
			users = append(users, *userStats)
		}
	}

	ctx.JSON(http.StatusOK, wrapResponse(UsersStatsResponse{
		Users: users,
	}))
}

func (c *StatsController) handleGetUserOnlineStatus(ctx *gin.Context) {
	var req UsernameRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserOnlineResponse{
			IsOnline: false,
		}))
		return
	}

	stm := c.getStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserOnlineResponse{
			IsOnline: false,
		}))
		return
	}

	counterName := "user>>>" + req.Username + ">>>online"
	value := c.getCounterValue(stm, counterName, false)

	ctx.JSON(http.StatusOK, wrapResponse(UserOnlineResponse{
		IsOnline: value > 0,
	}))
}

func (c *StatsController) handleGetUserIPList(ctx *gin.Context) {
	var req UserIDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserIPListResponse{
			IPs: []string{},
		}))
		return
	}

	stm := c.getConcreteStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserIPListResponse{
			IPs: []string{},
		}))
		return
	}

	onlineMap := stm.GetOnlineMap("user>>>" + req.UserID + ">>>online")
	if onlineMap == nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserIPListResponse{
			IPs: []string{},
		}))
		return
	}

	ipTimeMap := onlineMap.IpTimeMap()
	ips := make([]string, 0, len(ipTimeMap))
	for ip := range ipTimeMap {
		ips = append(ips, ip)
	}

	ctx.JSON(http.StatusOK, wrapResponse(UserIPListResponse{
		IPs: ips,
	}))
}

func (c *StatsController) handleGetInboundStats(ctx *gin.Context) {
	var req TagResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error(logFailedToParseInboundStatsRequest)
		ctx.JSON(http.StatusOK, wrapResponse(InboundStatsResponse{
			Inbound:  "",
			Uplink:   0,
			Downlink: 0,
		}))
		return
	}

	stm := c.getStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(InboundStatsResponse{
			Inbound:  req.Tag,
			Uplink:   0,
			Downlink: 0,
		}))
		return
	}

	uplinkName := "inbound>>>" + req.Tag + ">>>traffic>>>uplink"
	downlinkName := "inbound>>>" + req.Tag + ">>>traffic>>>downlink"

	uplink := c.getCounterValue(stm, uplinkName, req.Reset)
	downlink := c.getCounterValue(stm, downlinkName, req.Reset)

	ctx.JSON(http.StatusOK, wrapResponse(InboundStatsResponse{
		Inbound:  req.Tag,
		Uplink:   uplink,
		Downlink: downlink,
	}))
}

func (c *StatsController) handleGetOutboundStats(ctx *gin.Context) {
	var req TagResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error(logFailedToParseOutboundStatsRequest)
		ctx.JSON(http.StatusOK, wrapResponse(OutboundStatsResponse{
			Outbound: "",
			Uplink:   0,
			Downlink: 0,
		}))
		return
	}

	stm := c.getStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(OutboundStatsResponse{
			Outbound: req.Tag,
			Uplink:   0,
			Downlink: 0,
		}))
		return
	}

	uplinkName := "outbound>>>" + req.Tag + ">>>traffic>>>uplink"
	downlinkName := "outbound>>>" + req.Tag + ">>>traffic>>>downlink"

	uplink := c.getCounterValue(stm, uplinkName, req.Reset)
	downlink := c.getCounterValue(stm, downlinkName, req.Reset)

	ctx.JSON(http.StatusOK, wrapResponse(OutboundStatsResponse{
		Outbound: req.Tag,
		Uplink:   uplink,
		Downlink: downlink,
	}))
}

func (c *StatsController) handleGetAllInboundsStats(ctx *gin.Context) {
	var req ResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Reset = false
	}

	stm := c.getConcreteStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(AllInboundsStatsResponse{
			Inbounds: []InboundEntry{},
		}))
		return
	}

	trafficData := c.collectTrafficStats(stm, "inbound>>>", req.Reset)

	inbounds := make([]InboundEntry, 0, len(trafficData))
	for tag, traffic := range trafficData {
		inbounds = append(inbounds, InboundEntry{
			Inbound:  tag,
			Uplink:   traffic["uplink"],
			Downlink: traffic["downlink"],
		})
	}

	ctx.JSON(http.StatusOK, wrapResponse(AllInboundsStatsResponse{
		Inbounds: inbounds,
	}))
}

func (c *StatsController) handleGetAllOutboundsStats(ctx *gin.Context) {
	var req ResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Reset = false
	}

	stm := c.getConcreteStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(AllOutboundsStatsResponse{
			Outbounds: []OutboundEntry{},
		}))
		return
	}

	trafficData := c.collectTrafficStats(stm, "outbound>>>", req.Reset)

	outbounds := make([]OutboundEntry, 0, len(trafficData))
	for tag, traffic := range trafficData {
		outbounds = append(outbounds, OutboundEntry{
			Outbound: tag,
			Uplink:   traffic["uplink"],
			Downlink: traffic["downlink"],
		})
	}

	ctx.JSON(http.StatusOK, wrapResponse(AllOutboundsStatsResponse{
		Outbounds: outbounds,
	}))
}

func (c *StatsController) handleGetCombinedStats(ctx *gin.Context) {
	var req ResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Reset = false
	}

	stm := c.getConcreteStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(CombinedStatsResponse{
			Inbounds:  []InboundEntry{},
			Outbounds: []OutboundEntry{},
		}))
		return
	}

	inboundData := c.collectTrafficStats(stm, "inbound>>>", req.Reset)
	outboundData := c.collectTrafficStats(stm, "outbound>>>", req.Reset)

	inbounds := make([]InboundEntry, 0, len(inboundData))
	for tag, traffic := range inboundData {
		inbounds = append(inbounds, InboundEntry{
			Inbound:  tag,
			Uplink:   traffic["uplink"],
			Downlink: traffic["downlink"],
		})
	}

	outbounds := make([]OutboundEntry, 0, len(outboundData))
	for tag, traffic := range outboundData {
		outbounds = append(outbounds, OutboundEntry{
			Outbound: tag,
			Uplink:   traffic["uplink"],
			Downlink: traffic["downlink"],
		})
	}

	ctx.JSON(http.StatusOK, wrapResponse(CombinedStatsResponse{
		Inbounds:  inbounds,
		Outbounds: outbounds,
	}))
}
