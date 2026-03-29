// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package stoat

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sentinelb51/revoltgo"
)

// discordToRevolt maps a Discord permission value to the corresponding Revolt permission constant.
// Discord permissions not present in Revolt are omitted.
var discordToRevolt = map[int64]int64{
	discordgo.PermissionViewChannel:        int64(revoltgo.PermissionViewChannel),
	discordgo.PermissionSendMessages:       int64(revoltgo.PermissionSendMessage),
	discordgo.PermissionManageMessages:     int64(revoltgo.PermissionManageMessages),
	discordgo.PermissionEmbedLinks:         int64(revoltgo.PermissionSendEmbeds),
	discordgo.PermissionAttachFiles:        int64(revoltgo.PermissionUploadFiles),
	discordgo.PermissionReadMessageHistory: int64(revoltgo.PermissionReadMessageHistory),
	discordgo.PermissionAddReactions:       int64(revoltgo.PermissionReact),
	discordgo.PermissionVoiceConnect:       int64(revoltgo.PermissionConnect),
	discordgo.PermissionVoiceSpeak:         int64(revoltgo.PermissionSpeak),
	discordgo.PermissionVoiceMuteMembers:   int64(revoltgo.PermissionMuteMembers),
	discordgo.PermissionVoiceDeafenMembers: int64(revoltgo.PermissionDeafenMembers),
	discordgo.PermissionVoiceMoveMembers:   int64(revoltgo.PermissionMoveMembers),
	discordgo.PermissionChangeNickname:     int64(revoltgo.PermissionChangeNickname),
	discordgo.PermissionManageNicknames:    int64(revoltgo.PermissionManageNicknames),
	discordgo.PermissionManageRoles:        int64(revoltgo.PermissionManageRole),
	discordgo.PermissionManageWebhooks:     int64(revoltgo.PermissionManageWebhooks),
	discordgo.PermissionKickMembers:        int64(revoltgo.PermissionKickMembers),
	discordgo.PermissionBanMembers:         int64(revoltgo.PermissionBanMembers),
	discordgo.PermissionManageChannels:     int64(revoltgo.PermissionManageChannel),
	discordgo.PermissionManageServer:       int64(revoltgo.PermissionManageServer),
	discordgo.PermissionMentionEveryone:    int64(revoltgo.PermissionMentionEveryone),
}

// serverOnlyMask covers the server-only permission bits in Revolt (bits 1–13).
// These must not appear in channel permission overwrites or Revolt returns 422.
// Bit 0 (ManageChannel) is channel-valid and is intentionally excluded from the mask.
const serverOnlyMask int64 = (1 << 14) - 2 // bits 1–13 inclusive

// mapPermissions converts Discord allow/deny int64 permission bits to Revolt
// PermissionOverwrite Allow/Deny values. Unknown Discord bits are silently ignored.
// If the same bit appears in both discordAllow and discordDeny, both output words
// will have the corresponding Revolt bit set; the caller is responsible for resolving
// the conflict according to the target platform's precedence rules.
func mapPermissions(discordAllow, discordDeny int64) (allow, deny int64) {
	for discordPerm, revoltPerm := range discordToRevolt {
		if discordAllow&discordPerm != 0 {
			allow |= revoltPerm
		}
		if discordDeny&discordPerm != 0 {
			deny |= revoltPerm
		}
	}
	return
}

// mapChannelPermissions is like mapPermissions but strips server-only bits that
// are invalid in channel permission overwrites.
func mapChannelPermissions(discordAllow, discordDeny int64) (allow, deny int64) {
	allow, deny = mapPermissions(discordAllow, discordDeny)
	allow &^= serverOnlyMask
	deny &^= serverOnlyMask
	return
}
