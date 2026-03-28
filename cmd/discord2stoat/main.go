// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/blubskye/discord2stoat/internal/config"
	"github.com/blubskye/discord2stoat/internal/debug"
	"github.com/blubskye/discord2stoat/internal/discord"
	"github.com/blubskye/discord2stoat/internal/pipeline"
	"github.com/blubskye/discord2stoat/internal/target"
	targetfluxer "github.com/blubskye/discord2stoat/internal/target/fluxer"
	targetstoat "github.com/blubskye/discord2stoat/internal/target/stoat"
	"github.com/blubskye/discord2stoat/internal/tui"
)

func main() {
	debugFlag := flag.Bool("debug", false, "enable verbose debug logging to discord2stoat.log")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: discord2stoat [--debug] [version]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	logFile, err := os.OpenFile("discord2stoat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	if *debugFlag {
		debug.Enabled = true
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("debug logging enabled")
	}

	if flag.NArg() > 0 && flag.Arg(0) == "version" {
		fmt.Printf("discord2stoat %s\n  commit:  %s\n  source:  https://github.com/blubskye/discord2stoat\n  license: AGPL-3.0\n",
			version, commit)
		return
	}

	cfg, err := config.Load("config.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	discordClient, err := discord.New(cfg.DiscordToken, cfg.DiscordServerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	guild, err := discordClient.FetchGuild()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching Discord guild: %v\n", err)
		os.Exit(1)
	}

	channels, err := discordClient.FetchChannels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching channels: %v\n", err)
		os.Exit(1)
	}

	targets := make(map[string]target.Target)
	confirmEntries := []tui.ConfirmEntry{
		{Label: "Source  (Discord)", Name: guild.Name, ServerID: cfg.DiscordServerID},
	}

	if cfg.Stoat != nil {
		a := targetstoat.New(cfg.Stoat.Token, cfg.Stoat.ServerID)
		targets["stoat"] = a
		confirmEntries = append(confirmEntries, tui.ConfirmEntry{
			Label:    "Target  (Stoat)",
			Name:     targetstoat.FetchServerName(cfg.Stoat.Token, cfg.Stoat.ServerID),
			ServerID: cfg.Stoat.ServerID,
		})
	}

	if cfg.Fluxer != nil {
		a, err := targetfluxer.New(cfg.Fluxer.Token, cfg.Fluxer.ServerID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		targets["fluxer"] = a
		confirmEntries = append(confirmEntries, tui.ConfirmEntry{
			Label:    "Target  (Fluxer)",
			Name:     targetfluxer.FetchServerName(cfg.Fluxer.Token, cfg.Fluxer.ServerID),
			ServerID: cfg.Fluxer.ServerID,
		})
	}

	appCfg := tui.AppConfig{
		ConfirmEntries: confirmEntries,
		Channels:       channels,
		Targets:        targets,
		Discord:        discordClient,
	}

	progressCh := make(chan pipeline.ProgressEvent, 200)
	pauser := pipeline.NewPauser()
	ctx, cancel := context.WithCancel(context.Background())

	app := tui.NewApp(appCfg)
	app.SetPipelineRunner(func(channelCfgs map[string]pipeline.ChannelConfig) {
		go func() {
			var runErr error
			defer func() {
				if runErr != nil {
					select {
					case progressCh <- pipeline.ProgressEvent{Kind: pipeline.EventPipelineError, Err: runErr}:
					default:
					}
				}
				close(progressCh)
			}()
			debug.Printf("pipeline starting")
			runErr = pipeline.Run(ctx, pipeline.RunConfig{
				Targets:     targets,
				Discord:     discordClient,
				ChannelCfgs: channelCfgs,
				ProgressCh:  progressCh,
				Pauser:      pauser,
			})
			if runErr != nil {
				log.Printf("pipeline error: %v", runErr)
			}
		}()
	})
	app.SetProgressCh(progressCh)
	app.SetPauser(pauser)
	app.SetCancel(cancel)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("TUI error: %v", err)
		cancel()
		os.Exit(1)
	}
	cancel()
}
