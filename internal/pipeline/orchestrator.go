// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/blubskye/discord2stoat/internal/target"
)

// DiscordFetcher is satisfied by discord.Client.
type DiscordFetcher interface {
	FetchGuild() (*discordgo.Guild, error)
	FetchRoles() ([]*discordgo.Role, error)
	FetchChannels() ([]*discordgo.Channel, error)
	FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
	FetchEmojis() ([]normalized.Emoji, error)
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

	emojis, err := cfg.Discord.FetchEmojis()
	if err != nil {
		log.Printf("fetch emojis: %v (skipping emoji clone)", err)
		emojis = nil
	}

	// Build skip set from user config (text/voice channels only; categories are always kept).
	skipChannelIDs := make(map[string]bool)
	for id, c := range cfg.ChannelCfgs {
		if c.Skip {
			skipChannelIDs[id] = true
		}
	}
	// Filter out skipped channels for Phase A (keep categories so layout is preserved).
	phaseAChannels := make([]*discordgo.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch.Type != discordgo.ChannelTypeGuildCategory && skipChannelIDs[ch.ID] {
			continue
		}
		phaseAChannels = append(phaseAChannels, ch)
	}

	// Phase A: run for each target concurrently.
	type phaseAOut struct {
		name string
		ids  IDMap
		err  error
	}
	paCh := make(chan phaseAOut, len(cfg.Targets))
	for name, t := range cfg.Targets {
		name, t := name, t
		go func() {
			result, err := RunPhaseA(ctx, t, name, roles, phaseAChannels, emojis, cfg.ProgressCh)
			if err != nil {
				paCh <- phaseAOut{name: name, err: err}
				return
			}
			paCh <- phaseAOut{name: name, ids: result.ChannelIDs}
		}()
	}
	channelMaps := make(map[string]IDMap)
	var phaseAErr error
	for range cfg.Targets {
		out := <-paCh
		if out.err != nil && phaseAErr == nil {
			phaseAErr = fmt.Errorf("phase A [%s]: %w", out.name, out.err)
		} else if out.err == nil {
			channelMaps[out.name] = out.ids
			log.Printf("Phase A complete for %s", out.name)
		}
	}
	if phaseAErr != nil {
		return phaseAErr
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
