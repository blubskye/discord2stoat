// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package discord

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
)

// Client wraps discordgo for reading a Discord server's structure and messages.
type Client struct {
	session  *discordgo.Session
	serverID string
}

// New creates a Discord client. token must include the "Bot " prefix.
func New(token, serverID string) (*Client, error) {
	s, err := discordgo.New(token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages
	return &Client{session: s, serverID: serverID}, nil
}

// FetchGuild returns the guild name and ID for confirmation display.
func (c *Client) FetchGuild() (*discordgo.Guild, error) {
	g, err := c.session.Guild(c.serverID)
	if err != nil {
		return nil, fmt.Errorf("discord: fetch guild: %w", err)
	}
	return g, nil
}

// FetchRoles returns all roles sorted by position ascending.
func (c *Client) FetchRoles() ([]*discordgo.Role, error) {
	roles, err := c.session.GuildRoles(c.serverID)
	if err != nil {
		return nil, fmt.Errorf("discord: fetch roles: %w", err)
	}
	return roles, nil
}

// FetchChannels returns all guild channels.
func (c *Client) FetchChannels() ([]*discordgo.Channel, error) {
	channels, err := c.session.GuildChannels(c.serverID)
	if err != nil {
		return nil, fmt.Errorf("discord: fetch channels: %w", err)
	}
	return channels, nil
}

// FetchEmojis returns all non-managed custom emojis for the guild,
// with their image data downloaded from the Discord CDN.
func (c *Client) FetchEmojis() ([]normalized.Emoji, error) {
	emojis, err := c.session.GuildEmojis(c.serverID)
	if err != nil {
		return nil, fmt.Errorf("fetch emojis: %w", err)
	}
	result := make([]normalized.Emoji, 0, len(emojis))
	for _, e := range emojis {
		if e.Managed {
			continue // skip integration-managed emojis (e.g. Twitch)
		}
		ext := "png"
		if e.Animated {
			ext = "gif"
		}
		url := fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.%s?size=128", e.ID, ext)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.Printf("discord: skip emoji %q: build request: %v", e.Name, err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("discord: skip emoji %q: download failed", e.Name)
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("discord: skip emoji %q: read body: %v", e.Name, err)
			continue
		}
		result = append(result, normalized.Emoji{
			Name:     e.Name,
			Data:     data,
			Animated: e.Animated,
		})
	}
	return result, nil
}

// FetchMessages fetches up to limit messages from channelID, oldest first.
// Pass limit=0 to fetch all messages.
func (c *Client) FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error {
	var beforeID string
	fetched := 0

	for {
		batchSize := 100
		if limit > 0 {
			remaining := limit - fetched
			if remaining <= 0 {
				break
			}
			if remaining < batchSize {
				batchSize = remaining
			}
		}

		msgs, err := c.session.ChannelMessages(channelID, batchSize, beforeID, "", "")
		if err != nil {
			return fmt.Errorf("discord: fetch messages channel=%s: %w", channelID, err)
		}
		if len(msgs) == 0 {
			break
		}

		// ChannelMessages returns newest-first; reverse for oldest-first delivery.
		for i := len(msgs) - 1; i >= 0; i-- {
			select {
			case <-done:
				return nil
			case out <- msgs[i]:
			}
		}

		fetched += len(msgs)
		beforeID = msgs[len(msgs)-1].ID // next page: older than last received
		if len(msgs) < batchSize {
			break
		}
	}
	return nil
}
