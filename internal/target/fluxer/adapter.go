package fluxer

import (
	"bytes"
	"fmt"

	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"
	"github.com/fluxergo/fluxergo/fluxer"
	"github.com/fluxergo/fluxergo/rest"
)

// Adapter implements target.Target for the Fluxer platform.
type Adapter struct {
	guilds   rest.Guilds
	channels rest.Channels
	guildID  snowflake.ID
}

// New creates a new Fluxer Adapter for the given guild.
func New(token, guildID string) (*Adapter, error) {
	id, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, fmt.Errorf("fluxer: invalid guild ID %q: %w", guildID, err)
	}
	client := rest.NewClient("Bot " + token)
	return &Adapter{
		guilds:   rest.NewGuilds(client),
		channels: rest.NewChannels(client, fluxer.AllowedMentions{}),
		guildID:  id,
	}, nil
}

// CreateRole creates a role on Fluxer and returns its string ID.
func (a *Adapter) CreateRole(r normalized.Role) (string, error) {
	perms := fluxer.Permissions(r.Permissions)
	role, err := a.guilds.CreateRole(a.guildID, fluxer.RoleCreate{
		Name:        r.Name,
		Color:       r.Color,
		Permissions: &perms,
		Hoist:       r.Hoist,
	})
	if err != nil {
		return "", fmt.Errorf("fluxer CreateRole %q: %w", r.Name, err)
	}
	return role.ID.String(), nil
}

// SetRoleOrder sets the order of roles on Fluxer.
func (a *Adapter) SetRoleOrder(roles []normalized.RoleOrder) error {
	updates := make([]fluxer.RolePositionUpdate, len(roles))
	for i, r := range roles {
		id, err := snowflake.Parse(r.RoleID)
		if err != nil {
			return fmt.Errorf("fluxer SetRoleOrder: invalid role ID %q: %w", r.RoleID, err)
		}
		pos := r.Position
		updates[i] = fluxer.RolePositionUpdate{
			ID:       id,
			Position: &pos,
		}
	}
	_, err := a.guilds.UpdateRolePositions(a.guildID, updates)
	if err != nil {
		return fmt.Errorf("fluxer SetRoleOrder: %w", err)
	}
	return nil
}

// CreateChannel creates a text, voice, or category channel on Fluxer.
// Channel.ParentID must be a Fluxer channel ID (already remapped by orchestrator).
func (a *Adapter) CreateChannel(c normalized.Channel) (string, error) {
	var create fluxer.GuildChannelCreate

	switch c.Type {
	case normalized.ChannelTypeCategory:
		create = fluxer.GuildCategoryChannelCreate{
			Name:     c.Name,
			Position: c.Position,
		}
	case normalized.ChannelTypeVoice:
		parentID, err := parseOptionalSnowflake(c.ParentID)
		if err != nil {
			return "", err
		}
		create = fluxer.GuildVoiceChannelCreate{
			Name:     c.Name,
			Position: c.Position,
			ParentID: parentID,
			NSFW:     c.NSFW,
		}
	default: // text
		parentID, err := parseOptionalSnowflake(c.ParentID)
		if err != nil {
			return "", err
		}
		create = fluxer.GuildTextChannelCreate{
			Name:     c.Name,
			Topic:    c.Topic,
			Position: c.Position,
			ParentID: parentID,
			NSFW:     c.NSFW,
		}
	}

	ch, err := a.guilds.CreateGuildChannel(a.guildID, create)
	if err != nil {
		return "", fmt.Errorf("fluxer CreateChannel %q: %w", c.Name, err)
	}
	return ch.ID().String(), nil
}

// SetChannelOrder sets the position and parent of all channels via a batch call.
func (a *Adapter) SetChannelOrder(updates []normalized.ChannelOrder) error {
	batch := make([]fluxer.GuildChannelPositionUpdate, 0, len(updates))
	for _, u := range updates {
		id, err := snowflake.Parse(u.ChannelID)
		if err != nil {
			return fmt.Errorf("fluxer SetChannelOrder: invalid channel ID %q: %w", u.ChannelID, err)
		}
		pos := u.Position
		upd := fluxer.GuildChannelPositionUpdate{
			ID:       id,
			Position: omit.NewPtr(pos),
		}
		if u.ParentID != "" {
			pid, err := snowflake.Parse(u.ParentID)
			if err != nil {
				return fmt.Errorf("fluxer SetChannelOrder: invalid parent ID %q: %w", u.ParentID, err)
			}
			upd.ParentID = &pid
		}
		batch = append(batch, upd)
	}
	return a.guilds.UpdateChannelPositions(a.guildID, batch)
}

// SetChannelPermissions sets role permission overwrites on a channel.
func (a *Adapter) SetChannelPermissions(channelID string, overwrites []normalized.Overwrite) error {
	cid, err := snowflake.Parse(channelID)
	if err != nil {
		return fmt.Errorf("fluxer SetChannelPermissions: invalid channel ID %q: %w", channelID, err)
	}
	for _, ow := range overwrites {
		rid, err := snowflake.Parse(ow.RoleID)
		if err != nil {
			return fmt.Errorf("fluxer SetChannelPermissions: invalid role ID %q: %w", ow.RoleID, err)
		}
		allow := fluxer.Permissions(ow.Allow)
		deny := fluxer.Permissions(ow.Deny)
		err = a.channels.UpdatePermissionOverwrite(cid, rid, fluxer.RolePermissionOverwriteUpdate{
			Allow: &allow,
			Deny:  &deny,
		})
		if err != nil {
			return fmt.Errorf("fluxer SetChannelPermissions channel=%s role=%s: %w", channelID, ow.RoleID, err)
		}
	}
	return nil
}

// SendMessage posts a message (with any attachments) to a Fluxer channel.
func (a *Adapter) SendMessage(channelID string, msg normalized.Message) error {
	cid, err := snowflake.Parse(channelID)
	if err != nil {
		return fmt.Errorf("fluxer SendMessage: invalid channel ID %q: %w", channelID, err)
	}
	content := msg.Content
	if msg.AuthorName != "" {
		content = fmt.Sprintf("[%s]: %s", msg.AuthorName, msg.Content)
	}
	// Skip if there is nothing to send.
	if content == "" && len(msg.Attachments) == 0 {
		return nil
	}
	mc := fluxer.MessageCreate{Content: content}
	for _, att := range msg.Attachments {
		mc = mc.AddFiles(fluxer.NewFile(att.Filename, "", bytes.NewReader(att.Data)))
	}
	_, err = a.channels.CreateMessage(cid, mc)
	if err != nil {
		return fmt.Errorf("fluxer SendMessage channel=%s: %w", channelID, err)
	}
	return nil
}

func parseOptionalSnowflake(id string) (snowflake.ID, error) {
	if id == "" {
		return 0, nil
	}
	parsed, err := snowflake.Parse(id)
	if err != nil {
		return 0, fmt.Errorf("fluxer: invalid channel ID %q: %w", id, err)
	}
	return parsed, nil
}
