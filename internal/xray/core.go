package xray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/routing"
	_ "github.com/xtls/xray-core/main/distro/all"

	"github.com/hteppl/remnawave-node-go/internal/logger"
)

func init() {
	if os.Getenv("XRAY_LOCATION_ASSET") == "" {
		for _, path := range []string{
			"/usr/local/share/xray",
			"/usr/share/xray",
			"/opt/xray",
			".",
		} {
			if _, err := os.Stat(path + "/geoip.dat"); err == nil {
				os.Setenv("XRAY_LOCATION_ASSET", path)
				break
			}
		}
	}
}

type Core struct {
	mu       sync.RWMutex
	instance *core.Instance
	logger   *logger.Logger
	running  bool
}

func NewCore(log *logger.Logger) *Core {
	return &Core{
		logger: log,
	}
}

func (c *Core) Start(configJSON []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		if err := c.stopLocked(); err != nil {
			return fmt.Errorf("failed to stop existing instance: %w", err)
		}
	}

	config, err := core.LoadConfig("json", bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	instance, err := core.New(config)
	if err != nil {
		return fmt.Errorf("failed to create xray instance: %w", err)
	}

	if err := instance.Start(); err != nil {
		instance.Close()
		return fmt.Errorf("failed to start xray: %w", err)
	}

	c.instance = instance
	c.running = true
	c.logger.Info("xray-core started successfully")

	return nil
}

func (c *Core) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopLocked()
}

func (c *Core) stopLocked() error {
	if c.instance == nil {
		return nil
	}

	if err := c.instance.Close(); err != nil {
		return fmt.Errorf("failed to close xray instance: %w", err)
	}

	c.instance = nil
	c.running = false
	c.logger.Info("xray-core stopped")

	return nil
}

func (c *Core) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Core) GetVersion() string {
	return core.Version()
}

func (c *Core) Instance() *core.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instance
}

func (c *Core) Restart(configJSON []byte) error {
	return c.Start(configJSON)
}

type routerWithRules interface {
	routing.Router
	AddRule(msg *serial.TypedMessage, shouldAppend bool) error
	RemoveRule(tag string) error
}

func (c *Core) getRouter() (routerWithRules, error) {
	c.mu.RLock()
	instance := c.instance
	c.mu.RUnlock()

	if instance == nil {
		return nil, fmt.Errorf("xray instance not running")
	}

	routerFeature := instance.GetFeature(routing.RouterType())
	if routerFeature == nil {
		return nil, fmt.Errorf("router feature not found")
	}

	r, ok := routerFeature.(routerWithRules)
	if !ok {
		return nil, fmt.Errorf("router does not support dynamic rule management")
	}

	return r, nil
}

func (c *Core) AddRoutingRule(ruleTag string, sourceIP string, outboundTag string) error {
	r, err := c.getRouter()
	if err != nil {
		return err
	}

	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", sourceIP)
	}

	var ipBytes []byte
	var prefix uint32
	if ip4 := ip.To4(); ip4 != nil {
		ipBytes = ip4
		prefix = 32
	} else {
		ipBytes = ip.To16()
		prefix = 128
	}

	routerConfig := &router.Config{
		Rule: []*router.RoutingRule{
			{
				RuleTag: ruleTag,
				TargetTag: &router.RoutingRule_Tag{
					Tag: outboundTag,
				},
				SourceGeoip: []*router.GeoIP{
					{
						Cidr: []*router.CIDR{
							{
								Ip:     ipBytes,
								Prefix: prefix,
							},
						},
					},
				},
			},
		},
	}

	typedMsg := serial.ToTypedMessage(routerConfig)

	if err := r.AddRule(typedMsg, true); err != nil {
		return fmt.Errorf("failed to add routing rule: %w", err)
	}

	c.logger.WithField("ruleTag", ruleTag).WithField("sourceIP", sourceIP).
		WithField("outbound", outboundTag).Info("Added routing rule")

	return nil
}

func (c *Core) RemoveRoutingRule(ruleTag string) error {
	r, err := c.getRouter()
	if err != nil {
		return err
	}

	if err := r.RemoveRule(ruleTag); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "empty tag") {
			c.logger.WithField("ruleTag", ruleTag).Warn("Rule not found, may already be removed")
			return nil
		}
		return fmt.Errorf("failed to remove routing rule: %w", err)
	}

	c.logger.WithField("ruleTag", ruleTag).Info("Removed routing rule")

	return nil
}

func ValidateConfig(configJSON []byte) error {
	var cfg map[string]interface{}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	_, err := core.LoadConfig("json", bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("invalid xray config: %w", err)
	}

	return nil
}
