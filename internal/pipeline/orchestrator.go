// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/target"
)

// DiscordFetcher is satisfied by discord.Client.
type DiscordFetcher interface {
	FetchGuild() (*discordgo.Guild, error)
	FetchRoles() ([]*discordgo.Role, error)
	FetchChannels() ([]*discordgo.Channel, error)
	FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
}

// RunConfig holds everything the orchestrator needs.
type RunConfig struct {
	Targets     map[string]target.Target // "stoat"/"fluxer" → adapter
	Discord     DiscordFetcher
	ChannelCfgs map[string]ChannelConfig // discordChannelID → user config
	ProgressCh  chan<- ProgressEvent
	Pauser      *Pauser
}

// Run executes Phase A (structure) then Phase B (messages) for all targets.
// Phase A runs sequentially; Phase B runs fully concurrently.
// Fatal Phase A errors abort the run. Per-channel Phase B errors are logged and skipped.
func Run(ctx context.Context, cfg RunConfig) error {
	// Fetch Discord data once.
	roles, err := cfg.Discord.FetchRoles()
	if err != nil {
		return fmt.Errorf("fetch roles: %w", err)
	}
	channels, err := cfg.Discord.FetchChannels()
	if err != nil {
		return fmt.Errorf("fetch channels: %w", err)
	}

	// Phase A: run for each target sequentially (order doesn't matter, but simpler).
	channelMaps := make(map[string]IDMap)
	for name, t := range cfg.Targets {
		result, err := RunPhaseA(ctx, t, name, roles, channels, cfg.ProgressCh)
		if err != nil {
			return fmt.Errorf("phase A [%s]: %w", name, err)
		}
		channelMaps[name] = result.ChannelIDs
		log.Printf("Phase A complete for %s", name)
	}

	// Collect text channels for Phase B.
	var textChannels []*discordgo.Channel
	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildText {
			textChannels = append(textChannels, ch)
		}
	}

	// Phase B: all channels concurrently across all targets.
	// RunPhaseB blocks until all per-channel goroutines complete.
	RunPhaseB(ctx, cfg.Discord, cfg.Targets, channelMaps, textChannels, cfg.ChannelCfgs, cfg.ProgressCh, cfg.Pauser)
	return nil
}
