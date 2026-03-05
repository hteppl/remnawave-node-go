package xray_test

import (
	"testing"

	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func TestConfigManager_IsNeedRestartCore_FirstStart(t *testing.T) {
	m := xray.NewConfigManager(nil)

	hashes := xray.Hashes{
		EmptyConfig: "abc123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}

	if !m.IsNeedRestartCore(hashes) {
		t.Error("First start should require restart")
	}
}

func TestConfigManager_IsNeedRestartCore_BaseConfigChanged(t *testing.T) {
	m := xray.NewConfigManager(nil)

	initialHashes := xray.Hashes{
		EmptyConfig: "original-hash",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(initialHashes, config)

	newHashes := xray.Hashes{
		EmptyConfig: "different-hash",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}

	if !m.IsNeedRestartCore(newHashes) {
		t.Error("Changed base config should require restart")
	}
}

func TestConfigManager_IsNeedRestartCore_InboundCountChanged(t *testing.T) {
	m := xray.NewConfigManager(nil)

	initialHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds: []xray.InboundHash{
			{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0},
			{Tag: "trojan-in", Hash: "0000000000000000", UsersCount: 0},
		},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
			map[string]interface{}{
				"tag":      "trojan-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(initialHashes, config)

	newHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}

	if !m.IsNeedRestartCore(newHashes) {
		t.Error("Changed inbound count should require restart")
	}
}

func TestConfigManager_IsNeedRestartCore_InboundNoLongerExists(t *testing.T) {
	m := xray.NewConfigManager(nil)

	initialHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds: []xray.InboundHash{
			{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0},
		},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(initialHashes, config)

	newHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "trojan-in", Hash: "0000000000000000", UsersCount: 0}},
	}

	if !m.IsNeedRestartCore(newHashes) {
		t.Error("Missing existing inbound should require restart")
	}
}

func TestConfigManager_IsNeedRestartCore_UserHashChanged(t *testing.T) {
	m := xray.NewConfigManager(nil)

	initialHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(initialHashes, config)

	newHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "differenthash123", UsersCount: 1}},
	}

	if !m.IsNeedRestartCore(newHashes) {
		t.Error("Changed user hash should require restart")
	}
}

func TestConfigManager_IsNeedRestartCore_NoRestartNeeded(t *testing.T) {
	m := xray.NewConfigManager(nil)

	initialHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(initialHashes, config)

	sameHashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}

	if m.IsNeedRestartCore(sameHashes) {
		t.Error("Identical config should not require restart")
	}
}

func TestConfigManager_ExtractUsersFromConfig(t *testing.T) {
	m := xray.NewConfigManager(nil)

	hashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds: []xray.InboundHash{
			{Tag: "vless-in", Hash: "somehash", UsersCount: 2},
		},
	}

	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag": "vless-in",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{"id": "uuid-1"},
						map[string]interface{}{"id": "uuid-2"},
					},
				},
			},
			map[string]interface{}{
				"tag": "ignored-inbound",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{"id": "uuid-3"},
					},
				},
			},
		},
	}

	err := m.ExtractUsersFromConfig(hashes, config)
	if err != nil {
		t.Fatalf("ExtractUsersFromConfig failed: %v", err)
	}

	vlessHash := m.GetInboundHash("vless-in")
	if vlessHash == "" {
		t.Error("vless-in should have a hash")
	}
	if vlessHash == "0000000000000000" {
		t.Error("vless-in hash should not be empty (has 2 users)")
	}

	ignoredHash := m.GetInboundHash("ignored-inbound")
	if ignoredHash != "" {
		t.Error("ignored-inbound should not be in hash map")
	}

	tags := m.GetXtlsConfigInbounds()
	if len(tags) != 1 {
		t.Errorf("Expected 1 inbound, got %d", len(tags))
	}
}

func TestConfigManager_AddRemoveUser(t *testing.T) {
	m := xray.NewConfigManager(nil)

	// Setup initial state
	hashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(hashes, config)

	emptyHash := m.GetInboundHash("vless-in")
	if emptyHash != "0000000000000000" {
		t.Errorf("Empty inbound should have zero hash, got %s", emptyHash)
	}

	m.AddUserToInbound("vless-in", "test-uuid")
	afterAddHash := m.GetInboundHash("vless-in")
	if afterAddHash == "0000000000000000" {
		t.Error("After adding user, hash should not be zero")
	}

	m.RemoveUserFromInbound("vless-in", "test-uuid")
	afterRemoveHash := m.GetInboundHash("vless-in")

	if afterRemoveHash != "" {
		t.Error("After removing last user, inbound should be removed from map")
	}
}

func TestConfigManager_AddUserToNewInbound(t *testing.T) {
	m := xray.NewConfigManager(nil)

	m.AddUserToInbound("new-inbound", "user-id")

	hash := m.GetInboundHash("new-inbound")
	if hash == "" {
		t.Error("New inbound should be created with user")
	}
	if hash == "0000000000000000" {
		t.Error("Inbound with user should not have zero hash")
	}
}

func TestConfigManager_Cleanup(t *testing.T) {
	m := xray.NewConfigManager(nil)

	hashes := xray.Hashes{
		EmptyConfig: "hash123",
		Inbounds:    []xray.InboundHash{{Tag: "vless-in", Hash: "0000000000000000", UsersCount: 0}},
	}
	config := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "vless-in",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	}
	_ = m.ExtractUsersFromConfig(hashes, config)

	if len(m.GetXtlsConfigInbounds()) == 0 {
		t.Error("Should have inbounds before cleanup")
	}

	m.Cleanup()

	if len(m.GetXtlsConfigInbounds()) != 0 {
		t.Error("Inbounds should be empty after cleanup")
	}
	if m.GetInboundHash("vless-in") != "" {
		t.Error("Hash map should be empty after cleanup")
	}

	if !m.IsNeedRestartCore(hashes) {
		t.Error("After cleanup, should need restart")
	}
}

func TestConfigManager_GetXrayConfig(t *testing.T) {
	m := xray.NewConfigManager(nil)

	cfg := m.GetXrayConfig()
	if cfg == nil {
		t.Error("GetXrayConfig should never return nil")
	}
	if len(cfg) != 0 {
		t.Error("Initial config should be empty")
	}

	m.SetXrayConfig(map[string]interface{}{"key": "value"})
	cfg = m.GetXrayConfig()
	if cfg["key"] != "value" {
		t.Error("Config should be retrievable")
	}
}
