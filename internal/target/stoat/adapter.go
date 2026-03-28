// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package stoat

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/sentinelb51/revoltgo"
)

// FetchServerName returns the display name of the Stoat server.
// Falls back to serverID on error.
// Uses raw HTTP instead of sess.Server() to avoid the uninitialized state map panic.
func FetchServerName(token, serverID string) string {
	sess := revoltgo.New(token)
	var server revoltgo.Server
	if err := sess.HTTP.Request(http.MethodGet, revoltgo.EndpointServer(serverID), nil, &server); err != nil {
		log.Printf("stoat: could not fetch server name for %s: %v", serverID, err)
		return serverID
	}
	return server.Name
}

// Adapter implements target.Target for the Stoat (Revolt) platform.
type Adapter struct {
	session  *revoltgo.Session
	serverID string

	// categoryMu guards the pending categories map.
	// CreateChannel stores categories here; SetChannelOrder flushes them.
	categoryMu sync.Mutex
	// pendingCategories maps local category ID → *revoltgo.ServerCategory being built.
	pendingCategories map[string]*revoltgo.ServerCategory
	// categoryOrder preserves insertion order for the final Categories array.
	categoryOrder []string
}

// New creates a new Stoat Adapter for the given server.
func New(token, serverID string) *Adapter {
	return &Adapter{
		session:           revoltgo.New(token),
		serverID:          serverID,
		pendingCategories: make(map[string]*revoltgo.ServerCategory),
	}
}

// roleCreateResponse captures the role ID returned by the Revolt API on creation.
// The library's ServersRoleCreate returns *ServerRole which has no ID field;
// the actual JSON response is {"id": "...", "role": {...}}.
type roleCreateResponse struct {
	ID string `json:"id"`
}

// CreateRole creates a role on Stoat. It sets name and rank on creation,
// then patches colour, hoist, and permissions separately.
func (a *Adapter) CreateRole(r normalized.Role) (string, error) {
	// Revolt rank: invert Discord position so higher Discord position = lower Revolt rank.
	rank := 1000 - r.Position

	// Use HTTP.Request directly so we can capture the role ID from the response.
	var resp roleCreateResponse
	endpoint := revoltgo.EndpointServerRoles(a.serverID)
	err := withRetry(func() error {
		return a.session.HTTP.Request(http.MethodPost, endpoint, revoltgo.ServerRoleCreateData{
			Name: r.Name,
			Rank: rank,
		}, &resp)
	})
	if err != nil {
		return "", fmt.Errorf("stoat CreateRole %q: %w", r.Name, err)
	}
	roleID := resp.ID

	colour := intToCSS(r.Color)
	hoist := r.Hoist
	_, err = withRetryVal(func() (*revoltgo.ServerRole, error) {
		return a.session.ServerRoleEdit(a.serverID, roleID, revoltgo.ServerRoleEditData{
			Colour: colour,
			Hoist:  &hoist,
		})
	})
	if err != nil {
		return "", fmt.Errorf("stoat ServerRoleEdit %q: %w", r.Name, err)
	}

	// Set role permissions via raw HTTP PATCH (ServerRoleEditData lacks Permissions).
	allow, _ := mapPermissions(r.Permissions, 0)
	type rolePermPatch struct {
		Permissions revoltgo.PermissionOverwrite `json:"permissions"`
	}
	roleEndpoint := revoltgo.EndpointServerRole(a.serverID, roleID)
	if err := withRetry(func() error {
		return a.session.HTTP.Request(http.MethodPatch, roleEndpoint, rolePermPatch{
			Permissions: revoltgo.PermissionOverwrite{Allow: allow},
		}, nil)
	}); err != nil {
		return "", fmt.Errorf("stoat set role permissions %q: %w", r.Name, err)
	}

	return roleID, nil
}

// SetRoleOrder is a no-op for Stoat because rank is set during CreateRole.
func (a *Adapter) SetRoleOrder(_ []normalized.RoleOrder) error {
	return nil
}

// CreateChannel creates a text or voice channel, or buffers a category for later flushing.
// For categories, returns a placeholder ID that is stored in pendingCategories.
func (a *Adapter) CreateChannel(c normalized.Channel) (string, error) {
	if c.Type == normalized.ChannelTypeCategory {
		return a.createCategory(c)
	}

	chType := revoltgo.ServerChannelCreateDataTypeText
	if c.Type == normalized.ChannelTypeVoice {
		chType = revoltgo.ServerChannelCreateDataTypeVoice
	}

	created, err := withRetryVal(func() (*revoltgo.Channel, error) {
		return a.session.ServerChannelCreate(a.serverID, revoltgo.ServerChannelCreateData{
			Type:        chType,
			Name:        c.Name,
			Description: c.Topic,
			NSFW:        c.NSFW,
		})
	})
	if err != nil {
		return "", fmt.Errorf("stoat CreateChannel %q: %w", c.Name, err)
	}
	return created.ID, nil
}

// createCategory buffers a category for inclusion in the ServerEditData.Categories call.
// Returns a generated placeholder ID (used by the orchestrator as a local key).
func (a *Adapter) createCategory(c normalized.Channel) (string, error) {
	// Stoat categories don't have a create endpoint — they're embedded in server metadata.
	// We generate a local ID so the orchestrator can track the mapping.
	localID := fmt.Sprintf("cat-%s", c.DiscordID)
	cat := &revoltgo.ServerCategory{
		ID:    localID,
		Title: c.Name,
	}
	a.categoryMu.Lock()
	a.pendingCategories[localID] = cat
	a.categoryOrder = append(a.categoryOrder, localID)
	a.categoryMu.Unlock()
	return localID, nil
}

// SetChannelOrder sets the final channel positions and flushes pending categories.
// For Stoat, categories are part of the server metadata. This call:
//  1. Builds the Categories array with channels in position order.
//  2. Calls ServerEdit to persist categories (which defines channel order).
func (a *Adapter) SetChannelOrder(updates []normalized.ChannelOrder) error {
	a.categoryMu.Lock()
	defer a.categoryMu.Unlock()

	// Reset channel lists in all pending categories.
	for _, cat := range a.pendingCategories {
		cat.Channels = nil
	}

	// Assign channels to their categories in position order.
	for _, u := range updates {
		if u.ParentID == "" {
			continue
		}
		cat, ok := a.pendingCategories[u.ParentID]
		if !ok {
			continue
		}
		cat.Channels = append(cat.Channels, u.ChannelID)
	}

	// Build ordered categories slice.
	cats := make([]*revoltgo.ServerCategory, 0, len(a.categoryOrder))
	for _, id := range a.categoryOrder {
		cats = append(cats, a.pendingCategories[id])
	}

	_, err := withRetryVal(func() (*revoltgo.Server, error) {
		return a.session.ServerEdit(a.serverID, revoltgo.ServerEditData{
			Categories: cats,
		})
	})
	if err != nil {
		return fmt.Errorf("stoat SetChannelOrder (server edit): %w", err)
	}
	return nil
}

// SetChannelPermissions sets role permission overwrites on a channel.
// Overwrite.RoleID must be a Stoat role ID (already remapped by orchestrator).
func (a *Adapter) SetChannelPermissions(channelID string, overwrites []normalized.Overwrite) error {
	for _, ow := range overwrites {
		if ow.RoleID == "" {
			continue
		}
		allow, deny := mapPermissions(ow.Allow, ow.Deny)
		if err := withRetry(func() error {
			return a.session.ChannelPermissionsSet(channelID, ow.RoleID, revoltgo.PermissionOverwrite{
				Allow: allow,
				Deny:  deny,
			})
		}); err != nil {
			return fmt.Errorf("stoat SetChannelPermissions channel=%s role=%s: %w", channelID, ow.RoleID, err)
		}
	}
	return nil
}

// CreateEmoji uploads a custom emoji to the Stoat server via Autumn CDN then the emoji API.
// The emoji ID on Revolt is the Autumn file ID of the uploaded image.
func (a *Adapter) CreateEmoji(e normalized.Emoji) error {
	// Upload image to Autumn CDN to get the file ID (which becomes the emoji ID).
	fa, err := withRetryVal(func() (*revoltgo.FileAttachment, error) {
		return a.session.AttachmentUpload(&revoltgo.File{
			Name:   e.Name + emojiExt(e.Animated),
			Reader: bytes.NewReader(e.Data),
		})
	})
	if err != nil {
		return fmt.Errorf("stoat CreateEmoji %q upload: %w", e.Name, err)
	}

	_, err = withRetryVal(func() (*revoltgo.Emoji, error) {
		return a.session.EmojiCreate(fa.ID, revoltgo.EmojiCreateData{
			Name: e.Name,
			Parent: &revoltgo.EmojiParent{
				Type: "Server",
				ID:   a.serverID,
			},
		})
	})
	if err != nil {
		return fmt.Errorf("stoat CreateEmoji %q: %w", e.Name, err)
	}
	return nil
}

func emojiExt(animated bool) string {
	if animated {
		return ".gif"
	}
	return ".png"
}

// SendMessage posts a message (with any attachments) to a Stoat channel.
// Each attachment is uploaded to Revolt's Autumn CDN first to get a file ID.
func (a *Adapter) SendMessage(channelID string, msg normalized.Message) error {
	content := msg.Content
	if msg.AuthorName != "" {
		content = fmt.Sprintf("[%s]: %s", msg.AuthorName, msg.Content)
	}

	// Upload attachments to Autumn CDN and collect IDs.
	// Failed uploads are logged and skipped; the message is still sent with remaining attachments.
	var fileIDs []string
	for _, att := range msg.Attachments {
		fa, err := withRetryVal(func() (*revoltgo.FileAttachment, error) {
			return a.session.AttachmentUpload(&revoltgo.File{
				Name:   att.Filename,
				Reader: bytes.NewReader(att.Data),
			})
		})
		if err != nil {
			log.Printf("stoat upload attachment %s: %v", att.Filename, err)
			continue
		}
		fileIDs = append(fileIDs, fa.ID)
	}

	// Skip entirely if there is nothing to send.
	if content == "" && len(fileIDs) == 0 {
		return nil
	}

	_, err := withRetryVal(func() (*revoltgo.Message, error) {
		return a.session.ChannelMessageSend(channelID, revoltgo.MessageSend{
			Content:     content,
			Attachments: fileIDs,
		})
	})
	if err != nil {
		return fmt.Errorf("stoat SendMessage channel=%s: %w", channelID, err)
	}
	return nil
}
