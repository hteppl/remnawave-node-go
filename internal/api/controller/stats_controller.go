package controller

import (
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appstats "github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/features/stats"

	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type ResetRequest struct {
	Reset bool `json:"reset"`
}

type UsernameRequest struct {
	Username string `json:"username" binding:"required"`
}

type TagResetRequest struct {
	Tag   string `json:"tag" binding:"required"`
	Reset bool   `json:"reset"`
}

type SystemStatsResponse struct {
	NumGoroutine int    `json:"numGoroutine"`
	NumGC        uint32 `json:"numGC"`
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"totalAlloc"`
	Sys          uint64 `json:"sys"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	LiveObjects  uint64 `json:"liveObjects"`
	Uptime       int64  `json:"uptime"`
}

type UserStats struct {
	Username string `json:"username"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type UsersStatsResponse struct {
	Users []UserStats `json:"users"`
}

type UserOnlineResponse struct {
	Online bool `json:"online"`
}

type InboundStatsResponse struct {
	Inbound  string `json:"inbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type OutboundStatsResponse struct {
	Outbound string `json:"outbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type InboundEntry struct {
	Inbound  string `json:"inbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type AllInboundsStatsResponse struct {
	Inbounds []InboundEntry `json:"inbounds"`
}

type OutboundEntry struct {
	Outbound string `json:"outbound"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type AllOutboundsStatsResponse struct {
	Outbounds []OutboundEntry `json:"outbounds"`
}

type CombinedStatsResponse struct {
	Inbounds  []InboundEntry  `json:"inbounds"`
	Outbounds []OutboundEntry `json:"outbounds"`
}

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
	group.POST("/get-inbound-stats", c.handleGetInboundStats)
	group.POST("/get-outbound-stats", c.handleGetOutboundStats)
	group.POST("/get-all-inbounds-stats", c.handleGetAllInboundsStats)
	group.POST("/get-all-outbounds-stats", c.handleGetAllOutboundsStats)
	group.POST("/get-combined-stats", c.handleGetCombinedStats)
}

func (c *StatsController) getStatsManager() stats.Manager {
	instance := c.core.Instance()
	if instance == nil {
		return nil
	}

	stmFeature := instance.GetFeature(stats.ManagerType())
	if stmFeature == nil {
		return nil
	}

	stm, ok := stmFeature.(stats.Manager)
	if !ok {
		return nil
	}

	return stm
}

func (c *StatsController) getConcreteStatsManager() *appstats.Manager {
	instance := c.core.Instance()
	if instance == nil {
		return nil
	}

	stmFeature := instance.GetFeature(stats.ManagerType())
	if stmFeature == nil {
		return nil
	}

	stm, ok := stmFeature.(*appstats.Manager)
	if !ok {
		return nil
	}

	return stm
}

func (c *StatsController) getCounterValue(stm stats.Manager, name string, reset bool) int64 {
	counter := stm.GetCounter(name)
	if counter == nil {
		return 0
	}
	value := counter.Value()
	if reset {
		counter.Set(0)
	}
	return value
}

func (c *StatsController) collectTrafficStats(stm *appstats.Manager, prefix string, reset bool) map[string]map[string]int64 {
	result := make(map[string]map[string]int64)

	stm.VisitCounters(func(name string, counter stats.Counter) bool {
		if !strings.HasPrefix(name, prefix) {
			return true
		}

		parts := strings.Split(name, ">>>")
		if len(parts) < 4 {
			return true
		}

		tag := parts[1]
		if parts[2] != "traffic" {
			return true
		}
		direction := parts[3]

		if result[tag] == nil {
			result[tag] = make(map[string]int64)
		}

		value := counter.Value()
		if reset {
			counter.Set(0)
		}

		result[tag][direction] = value
		return true
	})

	return result
}

func (c *StatsController) collectUserStats(stm *appstats.Manager, reset bool) map[string]*UserStats {
	userTraffic := make(map[string]*UserStats)

	stm.VisitCounters(func(name string, counter stats.Counter) bool {
		if !strings.HasPrefix(name, "user>>>") {
			return true
		}

		parts := strings.Split(name, ">>>")
		if len(parts) < 4 || parts[2] != "traffic" {
			return true
		}

		username := parts[1]
		direction := parts[3]

		value := counter.Value()
		if reset {
			counter.Set(0)
		}

		if userTraffic[username] == nil {
			userTraffic[username] = &UserStats{Username: username}
		}

		if direction == "uplink" {
			userTraffic[username].Uplink = value
		} else if direction == "downlink" {
			userTraffic[username].Downlink = value
		}

		return true
	})

	return userTraffic
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
		if userStats.Uplink > 0 || userStats.Downlink > 0 {
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
		c.logger.WithError(err).Error("Failed to parse get-user-online-status request")
		ctx.JSON(http.StatusBadRequest, wrapResponse(UserOnlineResponse{
			Online: false,
		}))
		return
	}

	stm := c.getStatsManager()
	if stm == nil {
		ctx.JSON(http.StatusOK, wrapResponse(UserOnlineResponse{
			Online: false,
		}))
		return
	}

	counterName := "user>>>" + req.Username + ">>>online"
	value := c.getCounterValue(stm, counterName, false)

	ctx.JSON(http.StatusOK, wrapResponse(UserOnlineResponse{
		Online: value > 0,
	}))
}

func (c *StatsController) handleGetInboundStats(ctx *gin.Context) {
	var req TagResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.WithError(err).Error("Failed to parse get-inbound-stats request")
		ctx.JSON(http.StatusBadRequest, wrapResponse(InboundStatsResponse{
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
		c.logger.WithError(err).Error("Failed to parse get-outbound-stats request")
		ctx.JSON(http.StatusBadRequest, wrapResponse(OutboundStatsResponse{
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
