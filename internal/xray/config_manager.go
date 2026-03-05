package xray

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hteppl/remnawave-node-go/internal/logger"
)

type InboundHash struct {
	Tag        string `json:"tag"`
	Hash       string `json:"hash"`
	UsersCount int    `json:"usersCount"`
}

type Hashes struct {
	EmptyConfig string        `json:"emptyConfig"`
	Inbounds    []InboundHash `json:"inbounds"`
}

type Internals struct {
	ForceRestart bool   `json:"forceRestart"`
	Hashes       Hashes `json:"hashes"`
}

type InboundSettings struct {
	Clients []struct {
		ID string `json:"id"`
	} `json:"clients"`
}

type Inbound struct {
	Tag      string          `json:"tag"`
	Settings InboundSettings `json:"settings"`
}

type XrayConfig struct {
	Inbounds []Inbound `json:"inbounds"`
}

type ConfigManager struct {
	mu                 sync.RWMutex
	xrayConfig         map[string]interface{}
	emptyConfigHash    string
	inboundsHashMap    map[string]*HashedSet
	xtlsConfigInbounds map[string]struct{}
	log                *logger.Logger
}

func NewConfigManager(log *logger.Logger) *ConfigManager {
	return &ConfigManager{
		xrayConfig:         nil,
		emptyConfigHash:    "",
		inboundsHashMap:    make(map[string]*HashedSet),
		xtlsConfigInbounds: make(map[string]struct{}),
		log:                log,
	}
}

func (m *ConfigManager) GetXrayConfig() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.xrayConfig == nil {
		return map[string]interface{}{}
	}
	return m.xrayConfig
}

func (m *ConfigManager) SetXrayConfig(config map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.xrayConfig = config
}

func (m *ConfigManager) IsNeedRestartCore(incomingHashes Hashes) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.emptyConfigHash == "" {
		return true
	}

	if incomingHashes.EmptyConfig != m.emptyConfigHash {
		if m.log != nil {
			m.log.Warn("Detected changes in Xray Core base configuration")
		}
		return true
	}

	if len(incomingHashes.Inbounds) != len(m.inboundsHashMap) {
		if m.log != nil {
			m.log.Warn("Number of Xray Core inbounds has changed")
		}
		return true
	}

	for inboundTag, usersSet := range m.inboundsHashMap {
		var incomingInbound *InboundHash
		for i := range incomingHashes.Inbounds {
			if incomingHashes.Inbounds[i].Tag == inboundTag {
				incomingInbound = &incomingHashes.Inbounds[i]
				break
			}
		}

		if incomingInbound == nil {
			if m.log != nil {
				m.log.WithField("inbound", inboundTag).
					Warn("Inbound no longer exists in Xray Core configuration")
			}
			return true
		}

		if usersSet.Hash64String() != incomingInbound.Hash {
			if m.log != nil {
				m.log.WithField("inbound", inboundTag).
					WithField("current", usersSet.Hash64String()).
					WithField("incoming", incomingInbound.Hash).
					Warn("User configuration changed for inbound")
			}
			return true
		}
	}

	if m.log != nil {
		m.log.Info("Xray Core configuration is up-to-date - no restart required")
	}

	return false
}

func (m *ConfigManager) ExtractUsersFromConfig(hashes Hashes, newConfig map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()

	m.emptyConfigHash = hashes.EmptyConfig
	m.xrayConfig = newConfig

	if m.log != nil {
		hashJSON, _ := json.Marshal(hashes)
		m.log.Info(fmt.Sprintf("Starting user extraction from inbounds... Hash payload: %s", string(hashJSON)))
	}

	inboundsRaw, ok := newConfig["inbounds"]
	if !ok {
		return nil
	}

	inboundsSlice, ok := inboundsRaw.([]interface{})
	if !ok {
		return nil
	}

	validTags := make(map[string]struct{})
	for _, h := range hashes.Inbounds {
		validTags[h.Tag] = struct{}{}
	}

	for _, inboundRaw := range inboundsSlice {
		inbound, ok := inboundRaw.(map[string]interface{})
		if !ok {
			continue
		}

		tag, ok := inbound["tag"].(string)
		if !ok || tag == "" {
			continue
		}

		if _, valid := validTags[tag]; !valid {
			continue
		}

		usersSet := NewHashedSet()

		if settings, ok := inbound["settings"].(map[string]interface{}); ok {
			if clients, ok := settings["clients"].([]interface{}); ok {
				for _, clientRaw := range clients {
					if client, ok := clientRaw.(map[string]interface{}); ok {
						if id, ok := client["id"].(string); ok && id != "" {
							usersSet.Add(id)
						}
					}
				}
			}
		}

		m.inboundsHashMap[tag] = usersSet
		m.xtlsConfigInbounds[tag] = struct{}{}

		if m.log != nil {
			m.log.Info(fmt.Sprintf("%s has %d users", tag, usersSet.Size()))
		}
	}

	return nil
}

func (m *ConfigManager) AddUserToInbound(inboundTag, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	usersSet, exists := m.inboundsHashMap[inboundTag]
	if !exists {
		if m.log != nil {
			m.log.WithField("inbound", inboundTag).
				Warn("Inbound not found in inboundsHashMap, creating new one")
		}
		usersSet = NewHashedSet()
		usersSet.Add(userID)
		m.inboundsHashMap[inboundTag] = usersSet
		return
	}

	usersSet.Add(userID)
}

func (m *ConfigManager) RemoveUserFromInbound(inboundTag, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	usersSet, exists := m.inboundsHashMap[inboundTag]
	if !exists {
		return
	}

	usersSet.Delete(userID)

	if usersSet.Size() == 0 {
		delete(m.xtlsConfigInbounds, inboundTag)
		delete(m.inboundsHashMap, inboundTag)

		if m.log != nil {
			m.log.WithField("inbound", inboundTag).
				Warn("Inbound has no users, clearing from inboundsHashMap")
		}
	}
}

func (m *ConfigManager) GetXtlsConfigInbounds() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make([]string, 0, len(m.xtlsConfigInbounds))
	for tag := range m.xtlsConfigInbounds {
		tags = append(tags, tag)
	}
	return tags
}

func (m *ConfigManager) GetInboundHash(inboundTag string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if usersSet, exists := m.inboundsHashMap[inboundTag]; exists {
		return usersSet.Hash64String()
	}
	return ""
}

func (m *ConfigManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanup()
}

func (m *ConfigManager) cleanup() {
	if m.log != nil {
		m.log.Info("Cleaning up config manager")
	}

	m.inboundsHashMap = make(map[string]*HashedSet)
	m.xtlsConfigInbounds = make(map[string]struct{})
	m.xrayConfig = nil
	m.emptyConfigHash = ""
}
