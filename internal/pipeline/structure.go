package pipeline

import (
	"context"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/blubskye/discord2stoat/internal/target"
)

// IDMap maps original Discord IDs to new target-platform IDs.
type IDMap map[string]string

// StructureResult holds the ID maps produced by Phase A.
type StructureResult struct {
	RoleIDs    IDMap // discordRoleID → targetRoleID
	ChannelIDs IDMap // discordChannelID → targetChannelID
}

// RunPhaseA creates all roles, categories, channels, and permissions on t.
// It sends EventRoleCreated and EventStructureDone events on progressCh.
func RunPhaseA(
	ctx context.Context,
	t target.Target,
	targetName string,
	roles []*discordgo.Role,
	channels []*discordgo.Channel,
	progressCh chan<- ProgressEvent,
) (*StructureResult, error) {
	result := &StructureResult{
		RoleIDs:    make(IDMap),
		ChannelIDs: make(IDMap),
	}

	// --- Step 1: Create roles (sorted by position ascending) ---
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Position < roles[j].Position
	})
	for _, r := range roles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Skip the @everyone role (it already exists on the target).
		if r.Name == "@everyone" {
			continue
		}
		newID, err := t.CreateRole(normalized.Role{
			DiscordID:   r.ID,
			Name:        r.Name,
			Color:       r.Color,
			Permissions: r.Permissions,
			Hoist:       r.Hoist,
			Position:    r.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateRole %q: %w", targetName, r.Name, err)
		}
		result.RoleIDs[r.ID] = newID
		progressCh <- ProgressEvent{
			Kind:       EventRoleCreated,
			TargetName: targetName,
			RolesTotal: len(roles),
		}
	}

	// --- Step 2: Create role order ---
	roleOrders := make([]normalized.RoleOrder, 0, len(result.RoleIDs))
	for _, r := range roles {
		if newID, ok := result.RoleIDs[r.ID]; ok {
			roleOrders = append(roleOrders, normalized.RoleOrder{
				RoleID:   newID,
				Position: r.Position,
			})
		}
	}
	if err := t.SetRoleOrder(roleOrders); err != nil {
		return nil, fmt.Errorf("phaseA[%s] SetRoleOrder: %w", targetName, err)
	}

	// --- Step 3: Create categories first ---
	for _, ch := range channels {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ch.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		newID, err := t.CreateChannel(normalized.Channel{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Type:      normalized.ChannelTypeCategory,
			Position:  ch.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateChannel category %q: %w", targetName, ch.Name, err)
		}
		result.ChannelIDs[ch.ID] = newID
	}

	// --- Step 4: Create text and voice channels ---
	// Sort by position to preserve order.
	sorted := make([]*discordgo.Channel, len(channels))
	copy(sorted, channels)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})
	for _, ch := range sorted {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var chType normalized.ChannelType
		switch ch.Type {
		case discordgo.ChannelTypeGuildText:
			chType = normalized.ChannelTypeText
		case discordgo.ChannelTypeGuildVoice:
			chType = normalized.ChannelTypeVoice
		default:
			continue // skip categories (already done) and unsupported types
		}

		// Remap parent ID.
		parentID := ""
		if ch.ParentID != "" {
			parentID = result.ChannelIDs[ch.ParentID] // may be "" if category wasn't created
		}

		newID, err := t.CreateChannel(normalized.Channel{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Type:      chType,
			Topic:     ch.Topic,
			NSFW:      ch.NSFW,
			Position:  ch.Position,
			ParentID:  parentID,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateChannel %q: %w", targetName, ch.Name, err)
		}
		result.ChannelIDs[ch.ID] = newID
	}

	// --- Step 5: Set channel order ---
	orders := make([]normalized.ChannelOrder, 0, len(sorted))
	for _, ch := range sorted {
		newID, ok := result.ChannelIDs[ch.ID]
		if !ok {
			continue
		}
		parentID := ""
		if ch.ParentID != "" {
			parentID = result.ChannelIDs[ch.ParentID]
		}
		orders = append(orders, normalized.ChannelOrder{
			ChannelID: newID,
			Position:  ch.Position,
			ParentID:  parentID,
		})
	}
	if err := t.SetChannelOrder(orders); err != nil {
		return nil, fmt.Errorf("phaseA[%s] SetChannelOrder: %w", targetName, err)
	}

	// --- Step 6: Set channel permission overwrites ---
	for _, ch := range channels {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		newChanID, ok := result.ChannelIDs[ch.ID]
		if !ok {
			continue
		}
		overwrites := make([]normalized.Overwrite, 0, len(ch.PermissionOverwrites))
		for _, ow := range ch.PermissionOverwrites {
			if ow.Type != discordgo.PermissionOverwriteTypeRole {
				continue // skip member overwrites; no user mapping
			}
			newRoleID := result.RoleIDs[ow.ID]
			if newRoleID == "" {
				continue
			}
			overwrites = append(overwrites, normalized.Overwrite{
				RoleID: newRoleID,
				Allow:  ow.Allow,
				Deny:   ow.Deny,
			})
		}
		if len(overwrites) == 0 {
			continue
		}
		if err := t.SetChannelPermissions(newChanID, overwrites); err != nil {
			return nil, fmt.Errorf("phaseA[%s] SetChannelPermissions %q: %w", targetName, ch.Name, err)
		}
	}

	progressCh <- ProgressEvent{Kind: EventStructureDone, TargetName: targetName}
	return result, nil
}
