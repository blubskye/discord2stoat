// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package target

import "github.com/blubskye/discord2stoat/internal/normalized"

// Target is implemented by each platform adapter (Stoat, Fluxer).
// All methods use target-platform IDs in their return values.
type Target interface {
	// CreateRole creates a role and returns the new platform-specific ID.
	CreateRole(r normalized.Role) (newID string, err error)
	// SetRoleOrder sets the ordering of all roles after they have been created.
	SetRoleOrder(roles []normalized.RoleOrder) error
	// CreateChannel creates a text, voice, or category channel and returns the new ID.
	// Channel.ParentID must already be a target-platform ID (remapped by orchestrator).
	CreateChannel(c normalized.Channel) (newID string, err error)
	// SetChannelOrder sets the final position and parent of all channels.
	SetChannelOrder(updates []normalized.ChannelOrder) error
	// SetChannelPermissions sets role permission overwrites on a channel.
	// Overwrite.RoleID must already be a target-platform role ID.
	SetChannelPermissions(channelID string, overwrites []normalized.Overwrite) error
	// SendMessage posts a single message to a channel.
	SendMessage(channelID string, msg normalized.Message) error
	// CreateEmoji uploads a custom emoji to the target server.
	// Errors are non-fatal; the orchestrator logs and continues.
	CreateEmoji(e normalized.Emoji) error
}
