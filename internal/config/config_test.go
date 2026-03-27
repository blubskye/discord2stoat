package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blubskye/discord2stoat/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_BothTargets(t *testing.T) {
	path := writeConfig(t, `
discord_token     = "Bot abc"
discord_server_id = "111"

[stoat]
token     = "stoat-tok"
server_id = "sss"

[fluxer]
token     = "fluxer-tok"
server_id = "fff"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DiscordToken != "Bot abc" {
		t.Errorf("got DiscordToken %q", cfg.DiscordToken)
	}
	if cfg.DiscordServerID != "111" {
		t.Errorf("got DiscordServerID %q", cfg.DiscordServerID)
	}
	if cfg.Stoat == nil || cfg.Stoat.Token != "stoat-tok" {
		t.Error("expected stoat config")
	}
	if cfg.Fluxer == nil || cfg.Fluxer.Token != "fluxer-tok" {
		t.Error("expected fluxer config")
	}
}

func TestLoad_NoTargets_Error(t *testing.T) {
	path := writeConfig(t, `
discord_token     = "Bot abc"
discord_server_id = "111"
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for no targets")
	}
}

func TestLoad_EmptyTokenSkipped(t *testing.T) {
	path := writeConfig(t, `
discord_token     = "Bot abc"
discord_server_id = "111"

[stoat]
token     = ""
server_id = "sss"

[fluxer]
token     = "fluxer-tok"
server_id = "fff"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Stoat != nil {
		t.Error("stoat with empty token should be nil")
	}
	if cfg.Fluxer == nil {
		t.Error("expected fluxer config")
	}
}
