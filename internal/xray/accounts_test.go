package xray

import (
	"testing"

	"github.com/xtls/xray-core/common/protocol"
)

func TestBuildVlessUser(t *testing.T) {
	user := BuildVlessUser("test@example.com", "550e8400-e29b-41d4-a716-446655440000", "xtls-rprx-vision", 0)

	if user == nil {
		t.Fatal("BuildVlessUser returned nil")
	}

	if user.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "test@example.com")
	}

	if user.Level != 0 {
		t.Errorf("Level = %d, want 0", user.Level)
	}

	if user.Account == nil {
		t.Error("Account is nil")
	}
}

func TestBuildVlessUser_EmptyFlow(t *testing.T) {
	user := BuildVlessUser("test@example.com", "550e8400-e29b-41d4-a716-446655440000", "", 0)

	if user == nil {
		t.Fatal("BuildVlessUser returned nil")
	}

	// Empty flow should still create valid user
	if user.Account == nil {
		t.Error("Account is nil")
	}
}

func TestBuildTrojanUser(t *testing.T) {
	user := BuildTrojanUser("test@example.com", "secret-password", 0)

	if user == nil {
		t.Fatal("BuildTrojanUser returned nil")
	}

	if user.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "test@example.com")
	}

	if user.Level != 0 {
		t.Errorf("Level = %d, want 0", user.Level)
	}

	if user.Account == nil {
		t.Error("Account is nil")
	}
}

func TestBuildShadowsocksUser(t *testing.T) {
	user := BuildShadowsocksUser("test@example.com", "ss-password", CipherTypeCHACHA20POLY1305, false, 0)

	if user == nil {
		t.Fatal("BuildShadowsocksUser returned nil")
	}

	if user.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "test@example.com")
	}

	if user.Level != 0 {
		t.Errorf("Level = %d, want 0", user.Level)
	}

	if user.Account == nil {
		t.Error("Account is nil")
	}
}

func TestBuildShadowsocksUser_WithIVCheck(t *testing.T) {
	user := BuildShadowsocksUser("test@example.com", "ss-password", CipherTypeAES256GCM, true, 0)

	if user == nil {
		t.Fatal("BuildShadowsocksUser returned nil")
	}

	if user.Account == nil {
		t.Error("Account is nil")
	}
}

func TestBuildUserForInbound_Vless(t *testing.T) {
	inbound := InboundUserData{
		Type: "vless",
		Tag:  "vless-in",
		Flow: "xtls-rprx-vision",
	}
	userData := UserData{
		UserID:    "user1",
		HashUUID:  "550e8400-e29b-41d4-a716-446655440000",
		VlessUUID: "550e8400-e29b-41d4-a716-446655440000",
	}

	user := BuildUserForInbound(inbound, userData)

	if user == nil {
		t.Fatal("BuildUserForInbound returned nil")
	}
	if user.Email != "user1" {
		t.Errorf("Email = %q, want %q", user.Email, "user1")
	}
}

func TestBuildUserForInbound_Trojan(t *testing.T) {
	inbound := InboundUserData{
		Type: "trojan",
		Tag:  "trojan-in",
	}
	userData := UserData{
		UserID:         "user1",
		TrojanPassword: "secret-password",
	}

	user := BuildUserForInbound(inbound, userData)

	if user == nil {
		t.Fatal("BuildUserForInbound returned nil")
	}
	if user.Email != "user1" {
		t.Errorf("Email = %q, want %q", user.Email, "user1")
	}
}

func TestBuildUserForInbound_Shadowsocks(t *testing.T) {
	inbound := InboundUserData{
		Type:       "shadowsocks",
		Tag:        "ss-in",
		CipherType: CipherTypeCHACHA20POLY1305,
		IVCheck:    false,
	}
	userData := UserData{
		UserID:     "user1",
		SSPassword: "ss-password",
	}

	user := BuildUserForInbound(inbound, userData)

	if user == nil {
		t.Fatal("BuildUserForInbound returned nil")
	}
	if user.Email != "user1" {
		t.Errorf("Email = %q, want %q", user.Email, "user1")
	}
}

func TestBuildUserForInbound_Unknown(t *testing.T) {
	inbound := InboundUserData{
		Type: "unknown",
		Tag:  "unknown-in",
	}
	userData := UserData{UserID: "user1"}

	user := BuildUserForInbound(inbound, userData)

	if user != nil {
		t.Error("BuildUserForInbound should return nil for unknown type")
	}
}

func TestParseCipherType(t *testing.T) {
	tests := []struct {
		input    string
		expected CipherType
	}{
		{"aes-128-gcm", CipherTypeAES128GCM},
		{"AES_128_GCM", CipherTypeAES128GCM},
		{"aes-256-gcm", CipherTypeAES256GCM},
		{"AES_256_GCM", CipherTypeAES256GCM},
		{"chacha20-poly1305", CipherTypeCHACHA20POLY1305},
		{"chacha20-ietf-poly1305", CipherTypeCHACHA20POLY1305},
		{"CHACHA20_POLY1305", CipherTypeCHACHA20POLY1305},
		{"xchacha20-poly1305", CipherTypeXCHACHA20POLY1305},
		{"xchacha20-ietf-poly1305", CipherTypeXCHACHA20POLY1305},
		{"XCHACHA20_POLY1305", CipherTypeXCHACHA20POLY1305},
		{"none", CipherTypeNone},
		{"NONE", CipherTypeNone},
		{"invalid", CipherTypeUnknown},
		{"", CipherTypeUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseCipherType(tc.input)
			if result != tc.expected {
				t.Errorf("ParseCipherType(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUserToMemoryUser(t *testing.T) {
	// Test that built users can be converted to MemoryUser
	// This is the operation done before AddUser

	testCases := []struct {
		name string
		user *protocol.User
	}{
		{
			name: "VLESS user",
			user: BuildVlessUser("vless@test.com", "550e8400-e29b-41d4-a716-446655440000", "", 0),
		},
		{
			name: "Trojan user",
			user: BuildTrojanUser("trojan@test.com", "password123", 0),
		},
		{
			name: "Shadowsocks user",
			user: BuildShadowsocksUser("ss@test.com", "password123", CipherTypeCHACHA20POLY1305, false, 0),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mUser, err := tc.user.ToMemoryUser()
			if err != nil {
				t.Errorf("ToMemoryUser() failed: %v", err)
			}
			if mUser == nil {
				t.Fatal("ToMemoryUser() returned nil")
			}
			if mUser.Email != tc.user.Email {
				t.Errorf("MemoryUser.Email = %q, want %q", mUser.Email, tc.user.Email)
			}
		})
	}
}
