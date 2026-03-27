package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DiscordToken    string `toml:"discord_token"`
	DiscordServerID string `toml:"discord_server_id"`
	Stoat           *PlatformConfig
	Fluxer          *PlatformConfig
}

type PlatformConfig struct {
	Token    string `toml:"token"`
	ServerID string `toml:"server_id"`
}

type rawConfig struct {
	DiscordToken    string          `toml:"discord_token"`
	DiscordServerID string          `toml:"discord_server_id"`
	Stoat           *PlatformConfig `toml:"stoat"`
	Fluxer          *PlatformConfig `toml:"fluxer"`
}

func Load(path string) (*Config, error) {
	var raw rawConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{
		DiscordToken:    raw.DiscordToken,
		DiscordServerID: raw.DiscordServerID,
	}

	if raw.Stoat != nil && raw.Stoat.Token != "" {
		cfg.Stoat = raw.Stoat
	}
	if raw.Fluxer != nil && raw.Fluxer.Token != "" {
		cfg.Fluxer = raw.Fluxer
	}

	if cfg.Stoat == nil && cfg.Fluxer == nil {
		return nil, errors.New("config: at least one target ([stoat] or [fluxer]) must have a non-empty token")
	}

	return cfg, nil
}
