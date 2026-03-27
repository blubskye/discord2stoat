package stoat

import (
	"testing"

	"github.com/sentinelb51/revoltgo"
)

func TestMapPermissions_ViewChannel(t *testing.T) {
	discord := int64(1 << 10) // discordgo.PermissionViewChannel
	allow, deny := mapPermissions(discord, 0)
	if allow&revoltgo.PermissionViewChannel == 0 {
		t.Error("expected Revolt PermissionViewChannel to be set in allow")
	}
	_ = deny
}

func TestMapPermissions_SendMessages(t *testing.T) {
	discord := int64(1 << 11) // discordgo.PermissionSendMessages
	allow, deny := mapPermissions(discord, 0)
	if allow&revoltgo.PermissionSendMessage == 0 {
		t.Error("expected Revolt PermissionSendMessage to be set in allow")
	}
	_ = deny
}

func TestMapPermissions_DenyOverride(t *testing.T) {
	allow, deny := mapPermissions(0, int64(1<<10))
	if deny&revoltgo.PermissionViewChannel == 0 {
		t.Error("expected Revolt PermissionViewChannel to be set in deny")
	}
	_ = allow
}

func TestMapPermissions_UnknownBitsIgnored(t *testing.T) {
	allow, deny := mapPermissions(int64(1<<62), 0)
	if allow != 0 || deny != 0 {
		t.Errorf("expected zero outputs for unknown bits, got allow=%d deny=%d", allow, deny)
	}
}
