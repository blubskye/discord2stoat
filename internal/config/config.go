package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

// Config holds all parsed configuration.
type Config struct {
	DiscordToken    string `toml:"discord_token"`
	DiscordServerID string `toml:"discord_server_id"`
	Stoat           *PlatformConfig
	Fluxer          *PlatformConfig
}

// PlatformConfig holds credentials for a single target platform.
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

// Load parses config.toml at path and returns a validated Config.
// Returns an error if no target platform is configured.
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

	if cfg.DiscordToken == "" {
		return nil, errors.New("config: discord_token is required")
	}
	if cfg.DiscordServerID == "" {
		return nil, errors.New("config: discord_server_id is required")
	}

	if cfg.Stoat == nil && cfg.Fluxer == nil {
		return nil, errors.New("config: at least one target ([stoat] or [fluxer]) must have a non-empty token")
	}

	return cfg, nil
}
