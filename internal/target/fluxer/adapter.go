// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package fluxer

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/blubskye/discord2stoat/internal/debug"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"
	"github.com/fluxergo/fluxergo/fluxer"
	"github.com/fluxergo/fluxergo/rest"
)

// structureDelay is the minimum pause between successive guild-mutating API calls.
const structureDelay = 500 * time.Millisecond

// burstLimit / burstWindow implement a client-side token-bucket for Fluxer.
// Fluxer never sends X-RateLimit-* headers, so the library's rate limiter always
// thinks remaining=1 and fires at full speed. After ~10 ops the server silently
// holds TCP connections (no 429, no response) until the library's 20s HTTP timeout
// fires — repeated up to MaxRetries=10 times = ~200s of silence per stall.
// By capping to burstLimit ops per burstWindow we stay under the undocumented limit.
const (
	burstLimit  = 8
	burstWindow = 30 * time.Second
)

// FetchServerName returns the display name of the Fluxer guild.
// Falls back to serverID on error.
func FetchServerName(token, serverID string) string {
	id, err := snowflake.Parse(serverID)
	if err != nil {
		log.Printf("fluxer: could not parse guild ID %s: %v", serverID, err)
		return serverID
	}
	guilds := rest.NewGuilds(rest.NewClient(token))
	guild, err := guilds.GetGuild(id, false)
	if err != nil {
		log.Printf("fluxer: could not fetch guild name for %s: %v", serverID, err)
		return serverID
	}
	return guild.Name
}

// Adapter implements target.Target for the Fluxer platform.
type Adapter struct {
	guilds   rest.Guilds
	channels rest.Channels
	emojis   rest.Emojis
	guildID  snowflake.ID

	// burstCount and burstWinStart implement the client-side token bucket.
	burstCount    int
	burstWinStart time.Time
}

// trackBurst increments the burst counter and sleeps out the remainder of the
// current window once burstLimit is reached.  Call this before every
// guild-mutating API operation (CreateRole, CreateChannel, CreateEmoji) so
// that we never fire more than burstLimit requests within burstWindow.
func (a *Adapter) trackBurst() {
	if a.burstCount == 0 {
		a.burstWinStart = time.Now()
	}
	a.burstCount++
	if a.burstCount >= burstLimit {
		elapsed := time.Since(a.burstWinStart)
		if wait := burstWindow - elapsed; wait > 0 {
			log.Printf("fluxer: burst limit reached (%d ops in %s), pausing %s",
				a.burstCount, elapsed.Round(time.Second), wait.Round(time.Second))
			time.Sleep(wait)
		}
		a.burstCount = 0
		a.burstWinStart = time.Now()
	}
}

// New creates a new Fluxer Adapter for the given guild.
func New(token, guildID string) (*Adapter, error) {
	id, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, fmt.Errorf("fluxer: invalid guild ID %q: %w", guildID, err)
	}
	// Inject a slog.Logger into the rate limiter so its internal waits appear in our log.
	// The library reads X-RateLimit-Remaining / X-RateLimit-Reset-After from every response
	// and sleeps the exact server-specified duration — fully dynamic.
	// In debug mode use DEBUG level to see every bucket operation; otherwise WARN-only.
	slogLevel := slog.LevelWarn
	if debug.Enabled {
		slogLevel = slog.LevelDebug
	}
	slogLogger := slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{Level: slogLevel}))
	client := rest.NewClient(token, rest.WithRateLimiterConfigOpts(rest.WithRateLimiterLogger(slogLogger)))
	return &Adapter{
		guilds:   rest.NewGuilds(client),
		channels: rest.NewChannels(client, fluxer.AllowedMentions{}),
		emojis:   rest.NewEmojis(client),
		guildID:  id,
	}, nil
}

// PurgeRoles deletes all non-@everyone roles from the Fluxer guild.
// This prevents duplicates when the tool is run multiple times.
func (a *Adapter) PurgeRoles() error {
	roles, err := a.guilds.GetRoles(a.guildID)
	if err != nil {
		return fmt.Errorf("fluxer PurgeRoles fetch: %w", err)
	}
	debug.Printf("[fluxer] purging %d existing roles...", len(roles))
	deleted := 0
	for _, role := range roles {
		if role.Name == "@everyone" {
			continue
		}
		debug.Printf("[fluxer] deleting existing role %q (id=%s)...", role.Name, role.ID)
		if err := withRetry(fmt.Sprintf("DeleteRole %q", role.Name), func() error {
			return a.guilds.DeleteRole(a.guildID, role.ID)
		}); err != nil {
			log.Printf("fluxer PurgeRoles: delete role %q: %v", role.Name, err)
			continue
		}
		deleted++
		time.Sleep(structureDelay)
	}
	debug.Printf("[fluxer] purged %d roles", deleted)
	return nil
}

// PurgeChannels deletes all existing channels from the Fluxer guild.
func (a *Adapter) PurgeChannels() error {
	chs, err := a.guilds.GetGuildChannels(a.guildID)
	if err != nil {
		return fmt.Errorf("fluxer PurgeChannels fetch: %w", err)
	}
	debug.Printf("[fluxer] purging %d existing channels...", len(chs))
	deleted := 0
	for _, ch := range chs {
		id := ch.ID()
		debug.Printf("[fluxer] deleting existing channel %q (id=%s)...", ch.Name(), id)
		if err := withRetry(fmt.Sprintf("DeleteChannel %q", ch.Name()), func() error {
			return a.channels.DeleteChannel(id)
		}); err != nil {
			log.Printf("fluxer PurgeChannels: delete channel %q: %v", ch.Name(), err)
			continue
		}
		deleted++
		time.Sleep(structureDelay)
	}
	debug.Printf("[fluxer] purged %d channels", deleted)
	return nil
}

// CreateRole creates a role on Fluxer and returns its string ID.
func (a *Adapter) CreateRole(r normalized.Role) (string, error) {
	perms := fluxer.Permissions(r.Permissions)
	a.trackBurst()
	role, err := withRetryVal(fmt.Sprintf("CreateRole %q", r.Name), func() (*fluxer.Role, error) {
		return a.guilds.CreateRole(a.guildID, fluxer.RoleCreate{
			Name:        r.Name,
			Color:       r.Color,
			Permissions: &perms,
			Hoist:       r.Hoist,
		})
	})
	if err != nil {
		return "", fmt.Errorf("fluxer CreateRole %q: %w", r.Name, err)
	}
	time.Sleep(structureDelay)
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
	case normalized.ChannelTypeText:
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
	default:
		return "", fmt.Errorf("fluxer CreateChannel: unsupported channel type %d", c.Type)
	}

	a.trackBurst()
	ch, err := withRetryVal(fmt.Sprintf("CreateChannel %q", c.Name), func() (fluxer.GuildChannel, error) {
		return a.guilds.CreateGuildChannel(a.guildID, create)
	})
	if err != nil {
		return "", fmt.Errorf("fluxer CreateChannel %q: %w", c.Name, err)
	}
	time.Sleep(structureDelay)
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
	// Skip if there is nothing to send (check before building prefix).
	if msg.Content == "" && len(msg.Attachments) == 0 {
		return nil
	}
	content := msg.Content
	if msg.AuthorName != "" {
		content = fmt.Sprintf("[%s]: %s", msg.AuthorName, msg.Content)
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

// CreateEmoji creates a custom emoji in the Fluxer guild.
func (a *Adapter) CreateEmoji(e normalized.Emoji) error {
	iconType := fluxer.IconTypePNG
	if e.Animated {
		iconType = fluxer.IconTypeGIF
	}
	a.trackBurst()
	icon := fluxer.NewIconRaw(iconType, e.Data)
	_, err := withRetryVal(fmt.Sprintf("CreateEmoji %q", e.Name), func() (*fluxer.Emoji, error) {
		return a.emojis.CreateEmoji(a.guildID, fluxer.EmojiCreate{
			Name:  e.Name,
			Image: *icon,
		})
	})
	if err != nil {
		return fmt.Errorf("fluxer CreateEmoji %q: %w", e.Name, err)
	}
	time.Sleep(structureDelay)
	return nil
}

// parseOptionalSnowflake parses a snowflake ID string. An empty string returns
// the zero snowflake.ID (0), which the Fluxer API treats as "no parent".
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
