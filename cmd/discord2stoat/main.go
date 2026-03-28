package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/blubskye/discord2stoat/internal/config"
	"github.com/blubskye/discord2stoat/internal/discord"
	"github.com/blubskye/discord2stoat/internal/pipeline"
	"github.com/blubskye/discord2stoat/internal/target"
	targetfluxer "github.com/blubskye/discord2stoat/internal/target/fluxer"
	targetstoat "github.com/blubskye/discord2stoat/internal/target/stoat"
	"github.com/blubskye/discord2stoat/internal/tui"
)

func main() {
	logFile, err := os.OpenFile("discord2stoat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	if len(os.Args) > 1 && os.Args[1] == "version" {
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
			Name:     cfg.Stoat.ServerID,
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
			Name:     cfg.Fluxer.ServerID,
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
			defer close(progressCh)
			err := pipeline.Run(ctx, pipeline.RunConfig{
				Targets:     targets,
				Discord:     discordClient,
				ChannelCfgs: channelCfgs,
				ProgressCh:  progressCh,
				Pauser:      pauser,
			})
			if err != nil {
				log.Printf("pipeline error: %v", err)
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
