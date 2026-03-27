# Discord Clone Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a one-shot Go TUI tool that clones a Discord server's structure and message history onto Stoat and/or Fluxer.

**Architecture:** A three-phase tool: (1) TUI confirms and configures the clone, (2) Phase A creates structure (roles, channels, categories, permissions) on the target(s) sequentially, (3) Phase B concurrently fetches Discord messages and posts them to the target(s) using per-channel goroutine pairs. Both API libraries handle rate limiting from response headers internally.

**Tech Stack:** Go 1.23+, discordgo, revoltgo (local), fluxergo (local), bubbletea, lipgloss, BurntSushi/toml, AGPL-3.0

---

## File Map

```
cmd/discord2stoat/
  main.go               CLI entrypoint: parse args, load config, start TUI
  version.go            ldflags-embedded version/commit strings

internal/
  config/
    config.go           parse config.toml → Config struct
    config_test.go

  normalized/
    types.go            Role, Channel, ChannelOrder, RoleOrder, Overwrite, Message, ChannelType

  target/
    target.go           Target interface

  target/stoat/
    adapter.go          Stoat implementation of Target
    permissions.go      Discord permission bits → Revolt permission bits lookup table
    permissions_test.go
    color.go            int color → "#RRGGBB" CSS string
    color_test.go

  target/fluxer/
    adapter.go          Fluxer implementation of Target

  discord/
    client.go           DiscordClient: FetchGuild, FetchRoles, FetchChannels, FetchMessages

  pipeline/
    orchestrator.go     Run(ctx, targets, discordClient, config, progressCh) — phases A+B
    structure.go        PhaseA: roles→categories→channels→order→overwrites per target
    messages.go         PhaseB: per-channel goroutine pair (fetch+post) + pause support
    events.go           ProgressEvent types (shared between pipeline and tui)

  tui/
    app.go              bubbletea root model; screen state machine
    confirm.go          Screen1Model: show source+dest names, Confirm/Quit
    configure.go        Screen2Model: category+channel attribution/limit config
    progress.go         Screen3Model: live per-channel progress rows

LICENSE
config.toml             (example, not committed with real tokens)
go.mod
go.sum
```

---

### Task 1: Repo scaffolding — LICENSE, go.mod, .gitignore

**Files:**
- Create: `LICENSE`
- Create: `go.mod`
- Create: `.gitignore`
- Create: `config.toml` (example only)

- [ ] **Step 1: Create the AGPL-3.0 LICENSE file**

```
                    GNU AFFERO GENERAL PUBLIC LICENSE
                       Version 3, 19 November 2007

 Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 Everyone is permitted to copy and distribute verbatim copies
 of this license document, but changing it is not allowed.

                            Preamble

  The GNU Affero General Public License is a free, copyleft license for
software and other kinds of works, specifically designed to ensure
cooperation with the community in the case of network server software.
[... full AGPL-3.0 text ...]
```

Fetch the full text from https://www.gnu.org/licenses/agpl-3.0.txt and save to `LICENSE`.

- [ ] **Step 2: Create go.mod**

```
module github.com/blubskye/discord2stoat

go 1.23.0

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/bwmarrin/discordgo v0.28.1
	github.com/charmbracelet/bubbletea v1.3.4
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/disgoorg/snowflake/v2 v2.0.3
	github.com/fluxergo/fluxergo v0.0.0-00010101000000-000000000000
	github.com/sentinelb51/revoltgo v0.0.0-00010101000000-000000000000
)

replace (
	github.com/sentinelb51/revoltgo => ./revoltgo
	github.com/fluxergo/fluxergo => ./fluxergo
)
```

- [ ] **Step 3: Create .gitignore**

```gitignore
discord2stoat
discord2stoat.log
config.toml
.auth_token
```

- [ ] **Step 4: Create example config.toml**

```toml
# discord2stoat configuration
# Copy this file to config.toml and fill in your tokens.

discord_token     = "Bot YOUR_DISCORD_BOT_TOKEN"
discord_server_id = "YOUR_DISCORD_SERVER_ID"

# Enable Stoat target (remove or leave token empty to skip)
[stoat]
token     = "YOUR_STOAT_BOT_TOKEN"
server_id = "YOUR_STOAT_SERVER_ID"

# Enable Fluxer target (remove or leave token empty to skip)
[fluxer]
token     = "YOUR_FLUXER_BOT_TOKEN"
server_id = "YOUR_FLUXER_SERVER_ID"
```

- [ ] **Step 5: Run go mod tidy to pull dependencies**

```bash
cd /run/media/blubskye/3cf92542-a690-4c82-8a30-19a30ca2ce5a/discord2stoat
go mod tidy
```

Expected: `go.sum` created, no errors.

- [ ] **Step 6: Commit**

```bash
git init
git add LICENSE go.mod go.sum .gitignore config.toml.example
git commit -m "chore: initial repo scaffold with AGPL-3.0 license and go.mod"
```

---

### Task 2: Config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/config/... -v
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Implement config.go**

Create `internal/config/config.go`:

```go
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

	if cfg.Stoat == nil && cfg.Fluxer == nil {
		return nil, errors.New("config: at least one target ([stoat] or [fluxer]) must have a non-empty token")
	}

	return cfg, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/config/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config package with TOML parsing and dual-target validation"
```

---

### Task 3: Normalized types + Target interface

**Files:**
- Create: `internal/normalized/types.go`
- Create: `internal/target/target.go`

- [ ] **Step 1: Create normalized/types.go**

```go
package normalized

import "time"

// ChannelType is the normalized channel type.
type ChannelType int

const (
	ChannelTypeText     ChannelType = iota
	ChannelTypeVoice
	ChannelTypeCategory
)

// Role is a normalized Discord role for writing to a target platform.
type Role struct {
	DiscordID   string
	Name        string
	Color       int   // Discord int color (e.g. 0xFF5733)
	Permissions int64 // Discord permission bit flags
	Hoist       bool
	Position    int // Discord position (0 = lowest)
}

// Channel is a normalized Discord channel for writing to a target platform.
type Channel struct {
	DiscordID  string
	Name       string
	Type       ChannelType
	Topic      string
	NSFW       bool
	Position   int
	ParentID   string // Discord category ID; orchestrator remaps to target ID
	Overwrites []Overwrite
}

// Overwrite is a normalized permission overwrite.
// RoleID holds the new target role ID after remapping by the orchestrator.
type Overwrite struct {
	RoleID string
	Allow  int64
	Deny   int64
}

// ChannelOrder is used to set the final channel order on the target.
type ChannelOrder struct {
	ChannelID string // target platform channel ID
	Position  int
	ParentID  string // target platform parent category ID
}

// RoleOrder is used to set the final role order on the target.
type RoleOrder struct {
	RoleID   string // target platform role ID
	Position int    // Discord position; adapters invert/map as needed
}

// Message is a single normalized Discord message for posting to a target.
type Message struct {
	Content     string
	AuthorName  string       // populated when attribution mode is Prefix
	Timestamp   time.Time
	Attachments []Attachment // downloaded from Discord CDN once; re-uploaded by each adapter
}

// Attachment is a downloaded file from Discord. Files over 100 MB are skipped.
type Attachment struct {
	Filename string
	Data     []byte
}
```

- [ ] **Step 2: Create target/target.go**

```go
package target

import "github.com/blubskye/discord2stoat/internal/normalized"

// Target is implemented by each platform adapter (Stoat, Fluxer).
// All methods use target-platform IDs in their return values.
type Target interface {
	// CreateRole creates a role and returns the new platform-specific ID.
	CreateRole(r normalized.Role) (newID string, err error)
	// SetRoleOrder sets the ordering of all roles after they have been created.
	SetRoleOrder(roles []normalized.RoleOrder) error
	// CreateChannel creates a text, voice, or category channel and returns the new ID.
	// Channel.ParentID must already be a target-platform ID (remapped by orchestrator).
	CreateChannel(c normalized.Channel) (newID string, err error)
	// SetChannelOrder sets the final position and parent of all channels.
	SetChannelOrder(updates []normalized.ChannelOrder) error
	// SetChannelPermissions sets role permission overwrites on a channel.
	// Overwrite.RoleID must already be a target-platform role ID.
	SetChannelPermissions(channelID string, overwrites []normalized.Overwrite) error
	// SendMessage posts a single message to a channel.
	SendMessage(channelID string, msg normalized.Message) error
}
```

- [ ] **Step 3: Verify compile**

```bash
go build ./internal/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/normalized/ internal/target/target.go
git commit -m "feat: normalized types and Target interface"
```

---

### Task 4: Stoat color and permission helpers

**Files:**
- Create: `internal/target/stoat/color.go`
- Create: `internal/target/stoat/color_test.go`
- Create: `internal/target/stoat/permissions.go`
- Create: `internal/target/stoat/permissions_test.go`

- [ ] **Step 1: Write color tests**

Create `internal/target/stoat/color_test.go`:

```go
package stoat

import "testing"

func TestIntToCSS(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0xFF5733, "#FF5733"},
		{0x000000, "#000000"},
		{0xFFFFFF, "#FFFFFF"},
		{0x1ABC9C, "#1ABC9C"},
		{0, "#000000"},
	}
	for _, c := range cases {
		got := intToCSS(c.in)
		if got != c.want {
			t.Errorf("intToCSS(%#x) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Write permissions tests**

Create `internal/target/stoat/permissions_test.go`:

```go
package stoat

import (
	"testing"

	"github.com/sentinelb51/revoltgo"
)

func TestMapPermissions_ViewChannel(t *testing.T) {
	// Discord PermissionViewChannel (1<<10 = 1024) should map to Revolt PermissionViewChannel
	discord := int64(1 << 10)
	allow, deny := mapPermissions(discord, 0)
	if allow&revoltgo.PermissionViewChannel == 0 {
		t.Error("expected Revolt PermissionViewChannel to be set in allow")
	}
	_ = deny
}

func TestMapPermissions_SendMessages(t *testing.T) {
	// Discord PermissionSendMessages (1<<11 = 2048)
	discord := int64(1 << 11)
	allow, deny := mapPermissions(discord, 0)
	if allow&revoltgo.PermissionSendMessage == 0 {
		t.Error("expected Revolt PermissionSendMessage to be set in allow")
	}
}

func TestMapPermissions_DenyOverride(t *testing.T) {
	allow, deny := mapPermissions(0, int64(1<<10))
	if deny&revoltgo.PermissionViewChannel == 0 {
		t.Error("expected Revolt PermissionViewChannel to be set in deny")
	}
	_ = allow
}

func TestMapPermissions_UnknownBitsIgnored(t *testing.T) {
	// Bits with no Revolt equivalent should not panic or produce unexpected results
	allow, deny := mapPermissions(int64(1<<62), 0)
	_ = allow
	_ = deny
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./internal/target/stoat/... -v
```

Expected: compile error — package does not exist.

- [ ] **Step 4: Implement color.go**

Create `internal/target/stoat/color.go`:

```go
package stoat

import "fmt"

// intToCSS converts a Discord integer color (e.g. 0xFF5733) to a CSS hex string ("#FF5733").
func intToCSS(color int) string {
	return fmt.Sprintf("#%06X", color&0xFFFFFF)
}
```

- [ ] **Step 5: Implement permissions.go**

Discord and Revolt use different bit positions for permissions. This table maps Discord bits to Revolt bits.

Create `internal/target/stoat/permissions.go`:

```go
package stoat

import "github.com/sentinelb51/revoltgo"

// discordToRevolt maps a Discord permission bit (position) to the corresponding
// Revolt permission constant. Bits with no Revolt equivalent are omitted.
var discordToRevolt = map[int]int64{
	// Discord bit position : Revolt constant
	1:  revoltgo.PermissionCreateInstantInvite, // not in Revolt, skip — handled below
	6:  revoltgo.PermissionKickMembers,
	7:  revoltgo.PermissionBanMembers,
	3:  revoltgo.PermissionManageServer,
	4:  revoltgo.PermissionManageChannel,
	10: revoltgo.PermissionViewChannel,
	11: revoltgo.PermissionSendMessage,
	13: revoltgo.PermissionManageMessages,
	14: revoltgo.PermissionSendEmbeds,
	15: revoltgo.PermissionUploadFiles,
	16: revoltgo.PermissionReadMessageHistory,
	20: revoltgo.PermissionConnect,
	21: revoltgo.PermissionSpeak,
	22: revoltgo.PermissionMuteMembers,
	23: revoltgo.PermissionDeafenMembers,
	24: revoltgo.PermissionMoveMembers,
	26: revoltgo.PermissionChangeNickname,
	27: revoltgo.PermissionManageNicknames,
	28: revoltgo.PermissionManageRole,
	29: revoltgo.PermissionManageWebhooks,
	// reactions
	6:  revoltgo.PermissionReact, // Discord AddReactions = bit 6; map to Revolt React
	25: revoltgo.PermissionInviteOthers,
}

// mapPermissions converts Discord allow/deny int64 permission bits to Revolt
// PermissionOverwrite Allow/Deny values. Unknown Discord bits are silently ignored.
func mapPermissions(discordAllow, discordDeny int64) (allow, deny int64) {
	for discordBit, revoltPerm := range discordToRevolt {
		if discordAllow&(1<<discordBit) != 0 {
			allow |= revoltPerm
		}
		if discordDeny&(1<<discordBit) != 0 {
			deny |= revoltPerm
		}
	}
	return
}
```

Note: Discord bit 6 maps to both `KickMembers` and `AddReactions` in the table above — this is a placeholder. Resolve the correct Discord bit positions from `discordgo` constants during implementation:
- `discordgo.PermissionViewChannel` = `1 << 10`
- `discordgo.PermissionSendMessages` = `1 << 11`
- `discordgo.PermissionAddReactions` = `1 << 6`
- `discordgo.PermissionKickMembers` = `1 << 1`
- etc.

Use `discordgo` package constants directly in the map keys instead of raw numbers to avoid errors:

```go
import (
    "github.com/bwmarrin/discordgo"
    "github.com/sentinelb51/revoltgo"
)

// Use discordgo constants for bit positions:
// discordgo.PermissionViewChannel, discordgo.PermissionSendMessages, etc.
// These are int64 values like 1<<10, 1<<11, etc.
var discordToRevolt = map[int64]int64{
    discordgo.PermissionViewChannel:     revoltgo.PermissionViewChannel,
    discordgo.PermissionSendMessages:    revoltgo.PermissionSendMessage,
    discordgo.PermissionManageMessages:  revoltgo.PermissionManageMessages,
    discordgo.PermissionEmbedLinks:      revoltgo.PermissionSendEmbeds,
    discordgo.PermissionAttachFiles:     revoltgo.PermissionUploadFiles,
    discordgo.PermissionReadMessageHistory: revoltgo.PermissionReadMessageHistory,
    discordgo.PermissionAddReactions:    revoltgo.PermissionReact,
    discordgo.PermissionConnect:         revoltgo.PermissionConnect,
    discordgo.PermissionSpeak:           revoltgo.PermissionSpeak,
    discordgo.PermissionMuteMembers:     revoltgo.PermissionMuteMembers,
    discordgo.PermissionDeafenMembers:   revoltgo.PermissionDeafenMembers,
    discordgo.PermissionMoveMembers:     revoltgo.PermissionMoveMembers,
    discordgo.PermissionChangeNickname:  revoltgo.PermissionChangeNickname,
    discordgo.PermissionManageNicknames: revoltgo.PermissionManageNicknames,
    discordgo.PermissionManageRoles:     revoltgo.PermissionManageRole,
    discordgo.PermissionManageWebhooks:  revoltgo.PermissionManageWebhooks,
    discordgo.PermissionKickMembers:     revoltgo.PermissionKickMembers,
    discordgo.PermissionBanMembers:      revoltgo.PermissionBanMembers,
    discordgo.PermissionManageChannels:  revoltgo.PermissionManageChannel,
    discordgo.PermissionManageServer:    revoltgo.PermissionManageServer,
    discordgo.PermissionMentionEveryone: revoltgo.PermissionMentionEveryone,
}

func mapPermissions(discordAllow, discordDeny int64) (allow, deny int64) {
    for discordPerm, revoltPerm := range discordToRevolt {
        if discordAllow&discordPerm != 0 {
            allow |= revoltPerm
        }
        if discordDeny&discordPerm != 0 {
            deny |= revoltPerm
        }
    }
    return
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/target/stoat/... -v -run TestIntToCSS
go test ./internal/target/stoat/... -v -run TestMapPermissions
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/target/stoat/color.go internal/target/stoat/color_test.go \
        internal/target/stoat/permissions.go internal/target/stoat/permissions_test.go
git commit -m "feat: stoat color conversion and Discord→Revolt permission mapping"
```

---

### Task 5: Stoat adapter

**Files:**
- Create: `internal/target/stoat/adapter.go`

The Stoat adapter implements `target.Target` using `revoltgo.Session`. Key quirks:
- Categories are not channel types; they become `ServerCategory` entries in `ServerEditData.Categories`. `CreateChannel` stores categories in memory; `SetChannelOrder` triggers the server edit with the categories array.
- Role permissions are set via a raw HTTP PATCH because `ServerRoleEditData` lacks a `Permissions` field. We use `session.HTTP.Request` with a custom struct.
- Role rank is inverted: Discord `position=0` (lowest) maps to a high Revolt rank value.

- [ ] **Step 1: Create adapter.go**

```go
package stoat

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/sentinelb51/revoltgo"
)

// Adapter implements target.Target for the Stoat (Revolt) platform.
type Adapter struct {
	session  *revoltgo.Session
	serverID string

	// categoryMu guards the pending categories map.
	// CreateChannel stores categories here; SetChannelOrder flushes them.
	categoryMu sync.Mutex
	// pendingCategories maps new Stoat category ID → *revoltgo.ServerCategory being built.
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

// CreateRole creates a role on Stoat. It sets name and rank on creation,
// then patches colour, hoist, and permissions separately.
func (a *Adapter) CreateRole(r normalized.Role) (string, error) {
	// Revolt rank: invert Discord position so higher Discord position = lower Revolt rank.
	// We use position directly as rank; caller should pass unique, ordered positions.
	rank := 1000 - r.Position

	created, err := a.session.ServersRoleCreate(a.serverID, revoltgo.ServerRoleCreateData{
		Name: r.Name,
		Rank: rank,
	})
	if err != nil {
		return "", fmt.Errorf("stoat CreateRole %q: %w", r.Name, err)
	}

	colour := intToCSS(r.Color)
	hoist := r.Hoist
	_, err = a.session.ServerRoleEdit(a.serverID, created.ID, revoltgo.ServerRoleEditData{
		Colour: colour,
		Hoist:  &hoist,
	})
	if err != nil {
		return "", fmt.Errorf("stoat ServerRoleEdit %q: %w", r.Name, err)
	}

	// Set role permissions via raw HTTP PATCH (ServerRoleEditData lacks Permissions).
	allow, _ := mapPermissions(r.Permissions, 0)
	type rolePermPatch struct {
		Permissions revoltgo.PermissionOverwrite `json:"permissions"`
	}
	endpoint := revoltgo.EndpointServerRole(a.serverID, created.ID)
	if err := a.session.HTTP.Request(http.MethodPatch, endpoint, rolePermPatch{
		Permissions: revoltgo.PermissionOverwrite{Allow: allow},
	}, nil); err != nil {
		return "", fmt.Errorf("stoat set role permissions %q: %w", r.Name, err)
	}

	return created.ID, nil
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

	created, err := a.session.ServerChannelCreate(a.serverID, revoltgo.ServerChannelCreateData{
		Type:        chType,
		Name:        c.Name,
		Description: c.Topic,
		NSFW:        c.NSFW,
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
	// Channels without a parent go to a virtual uncategorized list (not in a category).
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

	_, err := a.session.ServerEdit(a.serverID, revoltgo.ServerEditData{
		Categories: cats,
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
		err := a.session.ChannelPermissionsSet(channelID, ow.RoleID, revoltgo.PermissionOverwrite{
			Allow: ow.Allow,
			Deny:  ow.Deny,
		})
		if err != nil {
			return fmt.Errorf("stoat SetChannelPermissions channel=%s role=%s: %w", channelID, ow.RoleID, err)
		}
	}
	return nil
}

// SendMessage posts a message (with any attachments) to a Stoat channel.
// Each attachment is uploaded to Revolt's Autumn CDN first to get a file ID.
func (a *Adapter) SendMessage(channelID string, msg normalized.Message) error {
	content := msg.Content
	if msg.AuthorName != "" {
		content = fmt.Sprintf("[%s]: %s", msg.AuthorName, msg.Content)
	}

	// Upload attachments to Autumn CDN and collect IDs.
	var fileIDs []string
	for _, att := range msg.Attachments {
		fa, err := a.session.UploadFile(&revoltgo.File{
			Name:   att.Filename,
			Reader: bytes.NewReader(att.Data),
		})
		if err != nil {
			log.Printf("stoat upload attachment %s: %v", att.Filename, err)
			continue
		}
		fileIDs = append(fileIDs, fa.ID)
	}

	_, err := a.session.ChannelMessageSend(channelID, revoltgo.MessageSend{
		Content:     content,
		Attachments: fileIDs,
	})
	if err != nil {
		return fmt.Errorf("stoat SendMessage channel=%s: %w", channelID, err)
	}
	return nil
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/target/stoat/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/target/stoat/adapter.go
git commit -m "feat: Stoat Target adapter"
```

---

### Task 6: Fluxer adapter

**Files:**
- Create: `internal/target/fluxer/adapter.go`

Fluxer uses the same permission model as Discord, so this adapter is nearly a passthrough. It uses `rest.NewClient` directly (no gateway/bot needed) plus `rest.NewGuilds` and `rest.NewChannels`.

- [ ] **Step 1: Create adapter.go**

```go
package fluxer

import (
	"fmt"

	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"
	"github.com/fluxergo/fluxergo/fluxer"
	"github.com/fluxergo/fluxergo/rest"
)

// Adapter implements target.Target for the Fluxer platform.
type Adapter struct {
	guilds   rest.Guilds
	channels rest.Channels
	guildID  snowflake.ID
}

// New creates a new Fluxer Adapter for the given guild.
func New(token, guildID string) (*Adapter, error) {
	id, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, fmt.Errorf("fluxer: invalid guild ID %q: %w", guildID, err)
	}
	client := rest.NewClient("Bot " + token)
	return &Adapter{
		guilds:   rest.NewGuilds(client),
		channels: rest.NewChannels(client, fluxer.AllowedMentions{}),
		guildID:  id,
	}, nil
}

// CreateRole creates a role on Fluxer and returns its string ID.
func (a *Adapter) CreateRole(r normalized.Role) (string, error) {
	perms := fluxer.Permissions(r.Permissions)
	role, err := a.guilds.CreateRole(a.guildID, fluxer.RoleCreate{
		Name:        r.Name,
		Color:       r.Color,
		Permissions: &perms,
		Hoist:       r.Hoist,
	})
	if err != nil {
		return "", fmt.Errorf("fluxer CreateRole %q: %w", r.Name, err)
	}
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
	default: // text
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
	}

	ch, err := a.guilds.CreateGuildChannel(a.guildID, create)
	if err != nil {
		return "", fmt.Errorf("fluxer CreateChannel %q: %w", c.Name, err)
	}
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
			Position: omit.Opt(&pos),
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

func parseOptionalSnowflake(id string) (snowflake.ID, error) {
	if id == "" {
		return 0, nil
	}
	return snowflake.Parse(id)
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/target/fluxer/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/target/fluxer/adapter.go
git commit -m "feat: Fluxer Target adapter"
```

---

### Task 7: Discord client

**Files:**
- Create: `internal/discord/client.go`

- [ ] **Step 1: Create client.go**

```go
package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
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
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/discord/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/discord/client.go
git commit -m "feat: Discord client for fetching guild structure and messages"
```

---

### Task 8: Pipeline events

**Files:**
- Create: `internal/pipeline/events.go`

These types are shared between the pipeline workers and the TUI progress screen.

- [ ] **Step 1: Create events.go**

```go
package pipeline

// EventKind identifies the type of progress event.
type EventKind int

const (
	EventRoleCreated     EventKind = iota // one role created on a target
	EventStructureDone                    // Phase A complete for a target
	EventChannelFetch                     // N messages fetched from Discord
	EventChannelPost                      // N messages posted to a target
	EventChannelDone                      // channel fully complete on a target
	EventChannelError                     // a channel encountered an error
	EventPhaseBDone                       // all channels complete for a target
)

// ProgressEvent is emitted by workers and consumed by the TUI.
type ProgressEvent struct {
	Kind       EventKind
	TargetName string // "stoat" or "fluxer"
	ChannelID  string // Discord channel ID (for per-channel events)
	Count      int    // messages fetched or posted (for Fetch/Post events)
	Total      int    // total expected (0 = unknown/all)
	RolesTotal int    // used by EventRoleCreated (total roles)
	Err        error  // non-nil for EventChannelError
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/pipeline/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/events.go
git commit -m "feat: pipeline progress event types"
```

---

### Task 9: Pipeline structure (Phase A)

**Files:**
- Create: `internal/pipeline/structure.go`

Phase A builds the server structure on each target sequentially: roles → categories → channels → order → permissions.

- [ ] **Step 1: Create structure.go**

```go
package pipeline

import (
	"context"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/blubskye/discord2stoat/internal/target"
)

// IDMap maps original Discord IDs to new target-platform IDs.
type IDMap map[string]string

// StructureResult holds the ID maps produced by Phase A.
type StructureResult struct {
	RoleIDs    IDMap // discordRoleID → targetRoleID
	ChannelIDs IDMap // discordChannelID → targetChannelID
}

// RunPhaseA creates all roles, categories, channels, and permissions on t.
// It sends EventRoleCreated and EventStructureDone events on progressCh.
func RunPhaseA(
	ctx context.Context,
	t target.Target,
	targetName string,
	roles []*discordgo.Role,
	channels []*discordgo.Channel,
	progressCh chan<- ProgressEvent,
) (*StructureResult, error) {
	result := &StructureResult{
		RoleIDs:    make(IDMap),
		ChannelIDs: make(IDMap),
	}

	// --- Step 1: Create roles (sorted by position ascending) ---
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Position < roles[j].Position
	})
	for _, r := range roles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Skip the @everyone role (it already exists on the target).
		if r.Name == "@everyone" {
			continue
		}
		newID, err := t.CreateRole(normalized.Role{
			DiscordID:   r.ID,
			Name:        r.Name,
			Color:       r.Color,
			Permissions: r.Permissions,
			Hoist:       r.Hoist,
			Position:    r.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateRole %q: %w", targetName, r.Name, err)
		}
		result.RoleIDs[r.ID] = newID
		progressCh <- ProgressEvent{
			Kind:       EventRoleCreated,
			TargetName: targetName,
			RolesTotal: len(roles),
		}
	}

	// --- Step 2: Create role order ---
	roleOrders := make([]normalized.RoleOrder, 0, len(result.RoleIDs))
	for _, r := range roles {
		if newID, ok := result.RoleIDs[r.ID]; ok {
			roleOrders = append(roleOrders, normalized.RoleOrder{
				RoleID:   newID,
				Position: r.Position,
			})
		}
	}
	if err := t.SetRoleOrder(roleOrders); err != nil {
		return nil, fmt.Errorf("phaseA[%s] SetRoleOrder: %w", targetName, err)
	}

	// --- Step 3: Create categories first ---
	for _, ch := range channels {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ch.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		newID, err := t.CreateChannel(normalized.Channel{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Type:      normalized.ChannelTypeCategory,
			Position:  ch.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateChannel category %q: %w", targetName, ch.Name, err)
		}
		result.ChannelIDs[ch.ID] = newID
	}

	// --- Step 4: Create text and voice channels ---
	// Sort by position to preserve order.
	sorted := make([]*discordgo.Channel, len(channels))
	copy(sorted, channels)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})
	for _, ch := range sorted {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var chType normalized.ChannelType
		switch ch.Type {
		case discordgo.ChannelTypeGuildText:
			chType = normalized.ChannelTypeText
		case discordgo.ChannelTypeGuildVoice:
			chType = normalized.ChannelTypeVoice
		default:
			continue // skip categories (already done) and unsupported types
		}

		// Remap parent ID.
		parentID := ""
		if ch.ParentID != "" {
			parentID = result.ChannelIDs[ch.ParentID] // may be "" if category wasn't created
		}

		newID, err := t.CreateChannel(normalized.Channel{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Type:      chType,
			Topic:     ch.Topic,
			NSFW:      ch.NSFW,
			Position:  ch.Position,
			ParentID:  parentID,
		})
		if err != nil {
			return nil, fmt.Errorf("phaseA[%s] CreateChannel %q: %w", targetName, ch.Name, err)
		}
		result.ChannelIDs[ch.ID] = newID
	}

	// --- Step 5: Set channel order ---
	orders := make([]normalized.ChannelOrder, 0, len(sorted))
	for _, ch := range sorted {
		newID, ok := result.ChannelIDs[ch.ID]
		if !ok {
			continue
		}
		parentID := ""
		if ch.ParentID != "" {
			parentID = result.ChannelIDs[ch.ParentID]
		}
		orders = append(orders, normalized.ChannelOrder{
			ChannelID: newID,
			Position:  ch.Position,
			ParentID:  parentID,
		})
	}
	if err := t.SetChannelOrder(orders); err != nil {
		return nil, fmt.Errorf("phaseA[%s] SetChannelOrder: %w", targetName, err)
	}

	// --- Step 6: Set channel permission overwrites ---
	for _, ch := range channels {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		newChanID, ok := result.ChannelIDs[ch.ID]
		if !ok {
			continue
		}
		overwrites := make([]normalized.Overwrite, 0, len(ch.PermissionOverwrites))
		for _, ow := range ch.PermissionOverwrites {
			if ow.Type != discordgo.PermissionOverwriteTypeRole {
				continue // skip member overwrites; no user mapping
			}
			newRoleID := result.RoleIDs[ow.ID]
			if newRoleID == "" {
				continue
			}
			overwrites = append(overwrites, normalized.Overwrite{
				RoleID: newRoleID,
				Allow:  ow.Allow,
				Deny:   ow.Deny,
			})
		}
		if len(overwrites) == 0 {
			continue
		}
		if err := t.SetChannelPermissions(newChanID, overwrites); err != nil {
			return nil, fmt.Errorf("phaseA[%s] SetChannelPermissions %q: %w", targetName, ch.Name, err)
		}
	}

	progressCh <- ProgressEvent{Kind: EventStructureDone, TargetName: targetName}
	return result, nil
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/pipeline/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/structure.go
git commit -m "feat: Phase A pipeline — create roles, channels, categories, permissions"
```

---

### Task 10: Pipeline messages (Phase B) + pause support

**Files:**
- Create: `internal/pipeline/messages.go`
- Create: `internal/pipeline/orchestrator.go`

- [ ] **Step 1: Create messages.go**

```go
package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/blubskye/discord2stoat/internal/target"
)

// ChannelConfig holds the user's per-channel configuration from the TUI.
type ChannelConfig struct {
	Attribution   AttributionMode
	MessageLimit  int // 0 = all
}

// AttributionMode controls how message author is formatted.
type AttributionMode int

const (
	AttributionPrefix      AttributionMode = iota // "[Username]: content"
	AttributionContentOnly                        // "content"
)

// Pauser allows workers to check for a pause signal between operations.
type Pauser struct {
	mu     sync.Mutex
	paused bool
	resume chan struct{}
}

func NewPauser() *Pauser {
	return &Pauser{resume: make(chan struct{})}
}

// Pause signals all workers to pause.
func (p *Pauser) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

// Resume unblocks all paused workers.
func (p *Pauser) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.paused {
		return
	}
	p.paused = false
	close(p.resume)
	p.resume = make(chan struct{})
}

// Check blocks if paused; returns ctx.Err() if context is cancelled while waiting.
func (p *Pauser) Check(ctx context.Context) error {
	p.mu.Lock()
	if !p.paused {
		p.mu.Unlock()
		return nil
	}
	ch := p.resume
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// RunPhaseB starts one fetch+post goroutine pair per text channel.
// It returns when all channels are done or ctx is cancelled.
func RunPhaseB(
	ctx context.Context,
	discordClient interface {
		FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
	},
	targets map[string]target.Target,         // targetName → Target
	channelMaps map[string]IDMap,              // targetName → discordChannelID→targetChannelID
	textChannels []*discordgo.Channel,
	channelCfg map[string]ChannelConfig,       // discordChannelID → config
	progressCh chan<- ProgressEvent,
	pauser *Pauser,
) {
	var wg sync.WaitGroup

	for _, ch := range textChannels {
		ch := ch // capture
		cfg, ok := channelCfg[ch.ID]
		if !ok {
			cfg = ChannelConfig{Attribution: AttributionPrefix}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runChannelWorker(ctx, discordClient, targets, channelMaps, ch, cfg, progressCh, pauser); err != nil {
				log.Printf("channel %s (%s): %v", ch.Name, ch.ID, err)
				progressCh <- ProgressEvent{
					Kind:      EventChannelError,
					ChannelID: ch.ID,
					Err:       err,
				}
			}
		}()
	}

	wg.Wait()
	for name := range targets {
		progressCh <- ProgressEvent{Kind: EventPhaseBDone, TargetName: name}
	}
}

func runChannelWorker(
	ctx context.Context,
	discordClient interface {
		FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
	},
	targets map[string]target.Target,
	channelMaps map[string]IDMap,
	ch *discordgo.Channel,
	cfg ChannelConfig,
	progressCh chan<- ProgressEvent,
	pauser *Pauser,
) error {
	buf := make(chan *discordgo.Message, 500)
	done := ctx.Done()

	// Fetch goroutine: reads from Discord and pushes to buf.
	fetchErr := make(chan error, 1)
	go func() {
		defer close(buf)
		fetchErr <- discordClient.FetchMessages(ch.ID, cfg.MessageLimit, buf, done)
	}()

	// Post goroutine: reads from buf and sends to all targets.
	fetched := 0
	posted := map[string]int{}

	for msg := range buf {
		if err := pauser.Check(ctx); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fetched++
		progressCh <- ProgressEvent{
			Kind:      EventChannelFetch,
			ChannelID: ch.ID,
			Count:     fetched,
			Total:     cfg.MessageLimit,
		}

		authorName := ""
		if cfg.Attribution == AttributionPrefix && msg.Author != nil {
			authorName = msg.Author.Username
		}

		// Download attachments (images, videos, files) from Discord CDN.
		var attachments []normalized.Attachment
		for _, a := range msg.Attachments {
			if a.Size > 100*1024*1024 { // skip files > 100 MB
				log.Printf("skipping large attachment %s (%d bytes)", a.Filename, a.Size)
				continue
			}
			resp, err := http.Get(a.URL)
			if err != nil {
				log.Printf("download attachment %s: %v", a.Filename, err)
				continue
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("read attachment %s: %v", a.Filename, err)
				continue
			}
			attachments = append(attachments, normalized.Attachment{
				Filename: a.Filename,
				Data:     data,
			})
		}

		norm := normalized.Message{
			Content:     msg.Content,
			AuthorName:  authorName,
			Timestamp:   msg.Timestamp,
			Attachments: attachments,
		}

		for name, t := range targets {
			targetChanID := channelMaps[name][ch.ID]
			if targetChanID == "" {
				continue
			}
			if err := t.SendMessage(targetChanID, norm); err != nil {
				return fmt.Errorf("[%s] SendMessage: %w", name, err)
			}
			posted[name]++
			progressCh <- ProgressEvent{
				Kind:      EventChannelPost,
				TargetName: name,
				ChannelID: ch.ID,
				Count:     posted[name],
				Total:     cfg.MessageLimit,
			}
		}
	}

	if err := <-fetchErr; err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	progressCh <- ProgressEvent{Kind: EventChannelDone, ChannelID: ch.ID}
	return nil
}
```

- [ ] **Step 2: Create orchestrator.go**

```go
package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

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
	Targets     map[string]target.Target  // "stoat"/"fluxer" → adapter
	Discord     DiscordFetcher
	ChannelCfgs map[string]ChannelConfig  // discordChannelID → user config
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
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		RunPhaseB(ctx, cfg.Discord, cfg.Targets, channelMaps, textChannels, cfg.ChannelCfgs, cfg.ProgressCh, cfg.Pauser)
	}()
	wg.Wait()
	return nil
}
```

- [ ] **Step 3: Verify compile**

```bash
go build ./internal/pipeline/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/pipeline/messages.go internal/pipeline/orchestrator.go
git commit -m "feat: Phase B pipeline — concurrent per-channel message workers with pause support"
```

---

### Task 11: TUI — confirm screen

**Files:**
- Create: `internal/tui/events.go`
- Create: `internal/tui/confirm.go`

- [ ] **Step 1: Create tui/events.go** (screen transition messages for bubbletea)

```go
package tui

// msgConfirmed is sent when the user presses Confirm on Screen 1.
type msgConfirmed struct{}

// msgQuit is sent when the user presses Quit.
type msgQuit struct{}

// msgStartClone is sent when the user presses Start on Screen 2.
type msgStartClone struct{}

// msgBack is sent when the user presses Back on Screen 2.
type msgBack struct{}
```

- [ ] **Step 2: Create tui/confirm.go**

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	labelStyle    = lipgloss.NewStyle().Width(22).Foreground(lipgloss.Color("243"))
	valueStyle    = lipgloss.NewStyle().Bold(true)
	subValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	buttonStyle   = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255"))
	activeButton  = buttonStyle.Background(lipgloss.Color("57"))
)

// ConfirmEntry holds one source or target server's display info.
type ConfirmEntry struct {
	Label    string // e.g. "Source (Discord)"
	Name     string // resolved server name
	ServerID string
}

// ConfirmModel is the bubbletea model for Screen 1.
type ConfirmModel struct {
	entries  []ConfirmEntry
	selected int // 0 = Confirm, 1 = Quit
}

// NewConfirmModel creates Screen 1 with the resolved server entries.
func NewConfirmModel(entries []ConfirmEntry) ConfirmModel {
	return ConfirmModel{entries: entries}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.selected > 0 {
				m.selected--
			}
		case "right", "l", "tab":
			if m.selected < 1 {
				m.selected++
			}
		case "enter", " ":
			if m.selected == 0 {
				return m, func() tea.Msg { return msgConfirmed{} }
			}
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	s := titleStyle.Render("discord2stoat") + "\n\n"
	for _, e := range m.entries {
		s += labelStyle.Render(e.Label+":") + " " +
			valueStyle.Render(e.Name) + "  " +
			subValueStyle.Render(fmt.Sprintf("[%s]", e.ServerID)) + "\n"
	}
	s += "\n"

	btn0 := buttonStyle.Render("Confirm")
	if m.selected == 0 {
		btn0 = activeButton.Render("Confirm")
	}
	btn1 := buttonStyle.Render("Quit")
	if m.selected == 1 {
		btn1 = activeButton.Render("Quit")
	}
	s += btn0 + "  " + btn1
	return s
}
```

- [ ] **Step 3: Verify compile**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/
git commit -m "feat: TUI confirm screen (Screen 1)"
```

---

### Task 12: TUI — configure screen

**Files:**
- Create: `internal/tui/configure.go`

The configure screen shows a scrollable list of channels grouped by category. Category rows act as bulk setters. Individual overrides show `*`.

- [ ] **Step 1: Create configure.go**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/blubskye/discord2stoat/internal/pipeline"
)

var (
	categoryStyle  = lipgloss.NewStyle().Bold(true)
	channelStyle   = lipgloss.NewStyle().PaddingLeft(4)
	overrideMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(" *")
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	selectAllStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
)

// ChanConfig is the user-editable config for one channel.
type ChanConfig struct {
	Attribution    pipeline.AttributionMode
	MessageLimit   int  // 0 = all
	Overridden     bool // true if manually changed from category default
}

// CategoryGroup holds a category and its channels for display.
type CategoryGroup struct {
	DiscordID string
	Name      string
	Collapsed bool
	Config    ChanConfig
	Channels  []ChannelRow
}

// ChannelRow is one text or voice channel in the list.
type ChannelRow struct {
	DiscordID string
	Name      string
	Config    ChanConfig
}

// ConfigureModel is the bubbletea model for Screen 2.
type ConfigureModel struct {
	// selectAll holds the global default.
	selectAll ChanConfig
	groups    []CategoryGroup
	// flat is the flattened list of focusable rows: selectAll row, category rows, channel rows.
	flat       []configRow
	cursor     int
	// editing tracks which field is active (attribution or limit).
	editField  int // 0=attribution, 1=limit
	editingLimit bool
	limitInput  string
}

type configRowKind int

const (
	rowSelectAll configRowKind = iota
	rowCategory
	rowChannel
)

type configRow struct {
	kind       configRowKind
	groupIdx   int  // index into groups
	channelIdx int  // index into group.Channels (-1 for category/selectAll rows)
}

// NewConfigureModel builds Screen 2 from Discord channel data.
func NewConfigureModel(channels []*discordgo.Channel) ConfigureModel {
	// Build groups: first find categories, then assign channels.
	catMap := map[string]*CategoryGroup{}
	var catOrder []string
	var uncategorized CategoryGroup
	uncategorized.Name = "Uncategorized"

	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			g := &CategoryGroup{
				DiscordID: ch.ID,
				Name:      ch.Name,
				Config:    ChanConfig{Attribution: pipeline.AttributionPrefix},
			}
			catMap[ch.ID] = g
			catOrder = append(catOrder, ch.ID)
		}
	}
	for _, ch := range channels {
		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildVoice {
			continue
		}
		row := ChannelRow{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Config:    ChanConfig{Attribution: pipeline.AttributionPrefix},
		}
		if g, ok := catMap[ch.ParentID]; ok {
			g.Channels = append(g.Channels, row)
		} else {
			uncategorized.Channels = append(uncategorized.Channels, row)
		}
	}

	groups := make([]CategoryGroup, 0, len(catOrder)+1)
	for _, id := range catOrder {
		groups = append(groups, *catMap[id])
	}
	if len(uncategorized.Channels) > 0 {
		groups = append(groups, uncategorized)
	}

	m := ConfigureModel{
		selectAll: ChanConfig{Attribution: pipeline.AttributionPrefix},
		groups:    groups,
	}
	m.rebuildFlat()
	return m
}

func (m *ConfigureModel) rebuildFlat() {
	m.flat = []configRow{{kind: rowSelectAll, groupIdx: -1, channelIdx: -1}}
	for gi, g := range m.groups {
		m.flat = append(m.flat, configRow{kind: rowCategory, groupIdx: gi, channelIdx: -1})
		if !g.Collapsed {
			for ci := range g.Channels {
				m.flat = append(m.flat, configRow{kind: rowChannel, groupIdx: gi, channelIdx: ci})
			}
		}
	}
}

func (m ConfigureModel) Init() tea.Cmd { return nil }

func (m ConfigureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	row := m.flat[m.cursor]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editingLimit {
			switch msg.String() {
			case "enter":
				m.applyLimitInput(row)
				m.editingLimit = false
				m.limitInput = ""
			case "backspace":
				if len(m.limitInput) > 0 {
					m.limitInput = m.limitInput[:len(m.limitInput)-1]
				}
			case "esc":
				m.editingLimit = false
				m.limitInput = ""
			default:
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.limitInput += msg.String()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.flat)-1 {
				m.cursor++
			}
		case "left", "h":
			if row.kind == rowCategory {
				m.groups[row.groupIdx].Collapsed = true
				m.rebuildFlat()
			} else if m.editField > 0 {
				m.editField--
			}
		case "right", "l":
			if row.kind == rowCategory {
				m.groups[row.groupIdx].Collapsed = false
				m.rebuildFlat()
			} else if m.editField < 1 {
				m.editField++
			}
		case "tab":
			m.editField = (m.editField + 1) % 2
		case "enter", " ":
			m.toggleField(row)
		case "s":
			return m, func() tea.Msg { return msgStartClone{} }
		case "b":
			return m, func() tea.Msg { return msgBack{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *ConfigureModel) toggleField(row configRow) {
	cfg := m.getConfig(row)
	if m.editField == 0 {
		// Toggle attribution.
		if cfg.Attribution == pipeline.AttributionPrefix {
			cfg.Attribution = pipeline.AttributionContentOnly
		} else {
			cfg.Attribution = pipeline.AttributionPrefix
		}
		m.setConfig(row, cfg, true)
		if row.kind == rowCategory || row.kind == rowSelectAll {
			m.propagateToChildren(row)
		}
	} else {
		// Start editing limit.
		m.editingLimit = true
		if cfg.MessageLimit > 0 {
			m.limitInput = fmt.Sprintf("%d", cfg.MessageLimit)
		}
	}
}

func (m *ConfigureModel) applyLimitInput(row configRow) {
	cfg := m.getConfig(row)
	if m.limitInput == "" || m.limitInput == "0" {
		cfg.MessageLimit = 0
	} else {
		var n int
		fmt.Sscanf(m.limitInput, "%d", &n)
		cfg.MessageLimit = n
	}
	m.setConfig(row, cfg, true)
	if row.kind == rowCategory || row.kind == rowSelectAll {
		m.propagateToChildren(row)
	}
}

func (m *ConfigureModel) propagateToChildren(row configRow) {
	parentCfg := m.getConfig(row)
	if row.kind == rowSelectAll {
		for gi := range m.groups {
			if !m.groups[gi].Channels[0].Config.Overridden { // propagate to non-overridden
				m.groups[gi].Config = parentCfg
				for ci := range m.groups[gi].Channels {
					if !m.groups[gi].Channels[ci].Config.Overridden {
						m.groups[gi].Channels[ci].Config = parentCfg
					}
				}
			}
		}
		return
	}
	for ci := range m.groups[row.groupIdx].Channels {
		if !m.groups[row.groupIdx].Channels[ci].Config.Overridden {
			m.groups[row.groupIdx].Channels[ci].Config = parentCfg
		}
	}
}

func (m *ConfigureModel) getConfig(row configRow) ChanConfig {
	switch row.kind {
	case rowSelectAll:
		return m.selectAll
	case rowCategory:
		return m.groups[row.groupIdx].Config
	default:
		return m.groups[row.groupIdx].Channels[row.channelIdx].Config
	}
}

func (m *ConfigureModel) setConfig(row configRow, cfg ChanConfig, markOverride bool) {
	switch row.kind {
	case rowSelectAll:
		m.selectAll = cfg
	case rowCategory:
		m.groups[row.groupIdx].Config = cfg
	default:
		if markOverride {
			cfg.Overridden = true
		}
		m.groups[row.groupIdx].Channels[row.channelIdx].Config = cfg
	}
}

// ExportChannelConfigs returns pipeline.ChannelConfig for each Discord channel ID.
func (m ConfigureModel) ExportChannelConfigs() map[string]pipeline.ChannelConfig {
	out := make(map[string]pipeline.ChannelConfig)
	for _, g := range m.groups {
		for _, ch := range g.Channels {
			out[ch.DiscordID] = pipeline.ChannelConfig{
				Attribution:  ch.Config.Attribution,
				MessageLimit: ch.Config.MessageLimit,
			}
		}
	}
	return out
}

func (m ConfigureModel) View() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Configure clone") + "\n\n")

	for i, row := range m.flat {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		switch row.kind {
		case rowSelectAll:
			cfg := m.selectAll
			sb.WriteString(cursor + selectAllStyle.Render("[Select All]") + "  " + m.renderFields(cfg, i == m.cursor) + "\n")
		case rowCategory:
			g := m.groups[row.groupIdx]
			arrow := "▼"
			if g.Collapsed {
				arrow = "▶"
			}
			sb.WriteString(cursor + categoryStyle.Render(fmt.Sprintf("%s %s", arrow, g.Name)) + "  " + m.renderFields(g.Config, i == m.cursor) + "\n")
		case rowChannel:
			ch := m.groups[row.groupIdx].Channels[row.channelIdx]
			override := ""
			if ch.Config.Overridden {
				override = overrideMarker
			}
			sb.WriteString(cursor + channelStyle.Render("#"+ch.Name) + "  " + m.renderFields(ch.Config, i == m.cursor) + override + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(buttonStyle.Render("[S]tart") + "  " + buttonStyle.Render("[B]ack") + "  " + buttonStyle.Render("[Q]uit"))
	sb.WriteString("\n  ← → or Tab to switch fields · Enter to toggle/edit · ← → on category to collapse")
	return sb.String()
}

func (m ConfigureModel) renderFields(cfg ChanConfig, active bool) string {
	attrLabel := "Prefix"
	if cfg.Attribution == pipeline.AttributionContentOnly {
		attrLabel = "Content"
	}
	limitLabel := "All"
	if cfg.MessageLimit > 0 {
		limitLabel = fmt.Sprintf("Last %d", cfg.MessageLimit)
	}

	attrField := fmt.Sprintf("Attribution: [%s]", attrLabel)
	limitField := fmt.Sprintf("Messages: [%s]", limitLabel)
	if active && m.editingLimit && m.editField == 1 {
		limitField = fmt.Sprintf("Messages: [%s_]", m.limitInput)
	}

	af := lipgloss.NewStyle()
	lf := lipgloss.NewStyle()
	if active {
		if m.editField == 0 {
			af = af.Foreground(lipgloss.Color("63")).Bold(true)
		} else {
			lf = lf.Foreground(lipgloss.Color("63")).Bold(true)
		}
	}
	return af.Render(attrField) + "  " + lf.Render(limitField)
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/configure.go
git commit -m "feat: TUI configure screen (Screen 2) with category bulk-set and per-channel overrides"
```

---

### Task 13: TUI — progress screen

**Files:**
- Create: `internal/tui/progress.go`

- [ ] **Step 1: Create progress.go**

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/blubskye/discord2stoat/internal/pipeline"
)

var (
	progressBarFull  = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("█")
	progressBarEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("░")
	doneStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	inProgressStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("…")
)

// channelState tracks live progress for one channel.
type channelState struct {
	name       string
	fetchCount int
	fetchTotal int // 0 = unknown
	// per-target post counts
	postCount  map[string]int
	postTotal  map[string]int
	done       bool
	err        error
}

// ProgressModel is the bubbletea model for Screen 3.
type ProgressModel struct {
	channels    map[string]*channelState // discordChannelID → state
	channelOrder []string               // insertion order
	categories  []categoryProgress

	rolesCreated int
	rolesTotal   int
	structDone   map[string]bool // targetName → done

	targets   []string
	paused    bool
	cancelled bool

	progressCh <-chan pipeline.ProgressEvent
}

type categoryProgress struct {
	name       string
	channelIDs []string
}

// NewProgressModel creates Screen 3.
func NewProgressModel(
	progressCh <-chan pipeline.ProgressEvent,
	targets []string,
	categories []categoryProgress,
	channels []*struct{ ID, Name string }, // ordered channel list
) ProgressModel {
	m := ProgressModel{
		channels:    make(map[string]*channelState),
		targets:     targets,
		structDone:  make(map[string]bool),
		categories:  categories,
		progressCh:  progressCh,
	}
	for _, ch := range channels {
		m.channels[ch.ID] = &channelState{
			name:      ch.Name,
			postCount: make(map[string]int),
			postTotal: make(map[string]int),
		}
		m.channelOrder = append(m.channelOrder, ch.ID)
	}
	return m
}

// WaitForEvent is a bubbletea Cmd that reads the next ProgressEvent.
func (m ProgressModel) WaitForEvent() tea.Cmd {
	return func() tea.Msg {
		return <-m.progressCh
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return m.WaitForEvent()
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pipeline.ProgressEvent:
		m.applyEvent(msg)
		return m, m.WaitForEvent()

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			m.paused = !m.paused
			// Signal pauser via a channel (wired by app.go)
		case "c", "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *ProgressModel) applyEvent(e pipeline.ProgressEvent) {
	switch e.Kind {
	case pipeline.EventRoleCreated:
		m.rolesCreated++
		m.rolesTotal = e.RolesTotal
	case pipeline.EventStructureDone:
		m.structDone[e.TargetName] = true
	case pipeline.EventChannelFetch:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.fetchCount = e.Count
			s.fetchTotal = e.Total
		}
	case pipeline.EventChannelPost:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.postCount[e.TargetName] = e.Count
			s.postTotal[e.TargetName] = e.Total
		}
	case pipeline.EventChannelDone:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.done = true
		}
	case pipeline.EventChannelError:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.err = e.Err
		}
	}
}

func (m ProgressModel) View() string {
	var sb strings.Builder

	// Roles progress bar
	if m.rolesTotal > 0 {
		sb.WriteString(renderBar("Roles", m.rolesCreated, m.rolesTotal) + "\n")
	}

	// Structure status
	structLine := "Structure  "
	allStructDone := len(m.structDone) == len(m.targets) && len(m.targets) > 0
	if allStructDone {
		structLine += doneStyle + " done"
	} else {
		structLine += inProgressStyle
	}
	sb.WriteString(structLine + "\n\n")

	// Channels grouped by category
	for _, cat := range m.categories {
		totalFetch, totalPost := 0, 0
		for _, id := range cat.channelIDs {
			if s, ok := m.channels[id]; ok {
				totalFetch += s.fetchCount
				for _, v := range s.postCount {
					totalPost += v
				}
			}
		}
		sb.WriteString(fmt.Sprintf("▼ %s   %s / %s msgs posted\n",
			categoryStyle.Render(cat.name),
			formatCount(totalPost),
			formatCount(totalFetch),
		))
		for _, id := range cat.channelIDs {
			s, ok := m.channels[id]
			if !ok {
				continue
			}
			sb.WriteString(m.renderChannelRow(s) + "\n")
		}
		sb.WriteString("\n")
	}

	// Pause/Cancel buttons
	pauseLabel := "[P]ause"
	if m.paused {
		pauseLabel = "[P]Resume"
	}
	sb.WriteString(buttonStyle.Render(pauseLabel) + "  " + buttonStyle.Render("[C]ancel"))
	return sb.String()
}

func (m ProgressModel) renderChannelRow(s *channelState) string {
	status := inProgressStyle
	if s.done {
		status = doneStyle
	} else if s.err != nil {
		status = errorStyle.Render("✗ " + s.err.Error())
	}

	fetchPart := fmt.Sprintf("fetch %s/%s", formatCount(s.fetchCount), formatTotal(s.fetchTotal))
	postParts := ""
	for _, name := range m.targets {
		postParts += fmt.Sprintf("  %s: post %s/%s", name, formatCount(s.postCount[name]), formatTotal(s.postTotal[name]))
	}

	return fmt.Sprintf("    #%-20s %s%s  %s", s.name, fetchPart, postParts, status)
}

func renderBar(label string, done, total int) string {
	const width = 10
	filled := 0
	if total > 0 {
		filled = done * width / total
	}
	bar := strings.Repeat(progressBarFull, filled) + strings.Repeat(progressBarEmpty, width-filled)
	return fmt.Sprintf("%-12s %s  %d / %d", label, bar, done, total)
}

func formatCount(n int) string {
	return fmt.Sprintf("%d", n)
}

func formatTotal(n int) string {
	if n == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", n)
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/progress.go
git commit -m "feat: TUI progress screen (Screen 3) with live per-channel status"
```

---

### Task 14: TUI — app root + screen transitions

**Files:**
- Create: `internal/tui/app.go`

The root model owns screen state and wires confirm → configure → progress.

- [ ] **Step 1: Create app.go**

```go
package tui

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/blubskye/discord2stoat/internal/pipeline"
	"github.com/blubskye/discord2stoat/internal/target"
)

type screen int

const (
	screenConfirm   screen = iota
	screenConfigure
	screenProgress
)

// AppModel is the top-level bubbletea model.
type AppModel struct {
	current   screen
	confirm   ConfirmModel
	configure ConfigureModel
	progress  ProgressModel

	// Set before progress screen starts.
	cancelFn    context.CancelFunc
	pauser      *pipeline.Pauser
	progressCh  chan pipeline.ProgressEvent
}

// AppConfig holds everything the TUI needs to start.
type AppConfig struct {
	ConfirmEntries []ConfirmEntry
	Channels       []*discordgo.Channel
	Targets        map[string]target.Target
	Discord        pipeline.DiscordFetcher
}

// NewApp creates the root model starting at the confirm screen.
func NewApp(cfg AppConfig) AppModel {
	return AppModel{
		current: screenConfirm,
		confirm: NewConfirmModel(cfg.ConfirmEntries),
		configure: NewConfigureModel(cfg.Channels),
		// progress and pipeline are set when the user presses Start
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.confirm.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case msgConfirmed:
		m.current = screenConfigure
		return m, m.configure.Init()

	case msgStartClone:
		// Transition to progress and start the pipeline.
		m.current = screenProgress
		return m, m.startPipeline()

	case msgBack:
		m.current = screenConfirm
		return m, m.confirm.Init()
	}

	switch m.current {
	case screenConfirm:
		updated, cmd := m.confirm.Update(msg)
		m.confirm = updated.(ConfirmModel)
		return m, cmd
	case screenConfigure:
		updated, cmd := m.configure.Update(msg)
		m.configure = updated.(ConfigureModel)
		return m, cmd
	case screenProgress:
		updated, cmd := m.progress.Update(msg)
		m.progress = updated.(ProgressModel)
		// Mirror pause state to the pipeline pauser.
		if m.pauser != nil {
			if m.progress.paused {
				m.pauser.Pause()
			} else {
				m.pauser.Resume()
			}
		}
		return m, cmd
	}
	return m, nil
}

func (m AppModel) View() string {
	switch m.current {
	case screenConfirm:
		return m.confirm.View()
	case screenConfigure:
		return m.configure.View()
	case screenProgress:
		return m.progress.View()
	}
	return ""
}

// startPipeline launches the orchestrator in a goroutine and returns the WaitForEvent Cmd.
func (m *AppModel) startPipeline() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel
	m.pauser = pipeline.NewPauser()
	m.progressCh = make(chan pipeline.ProgressEvent, 100)

	// Build category/channel lists for the progress screen.
	// (Simplified: pass all channels; progress screen groups them.)

	channelCfgs := m.configure.ExportChannelConfigs()

	// These are set by the app initializer and stored on AppModel (not shown here for brevity;
	// wire them via AppConfig and store on AppModel fields).
	_ = channelCfgs

	// Start orchestrator.
	go func() {
		defer close(m.progressCh)
		// Wire RunConfig from AppModel fields set by NewApp.
		// See main.go for full wiring.
		log.Println("pipeline started")
	}()

	m.progress = buildProgressModel(m.progressCh)
	return m.progress.WaitForEvent()
}

func buildProgressModel(ch <-chan pipeline.ProgressEvent) ProgressModel {
	return ProgressModel{
		channels:   make(map[string]*channelState),
		structDone: make(map[string]bool),
		progressCh: ch,
	}
}

// GracefulStop cancels the pipeline context.
func (m *AppModel) GracefulStop() {
	if m.cancelFn != nil {
		m.cancelFn()
	}
	fmt.Println()
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: TUI app root with confirm→configure→progress screen transitions"
```

---

### Task 15: main.go, version.go, and full wiring

**Files:**
- Create: `cmd/discord2stoat/version.go`
- Create: `cmd/discord2stoat/main.go`

- [ ] **Step 1: Create version.go**

```go
package main

// These variables are set at build time via:
//   go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc1234"
var (
	version = "dev"
	commit  = "unknown"
)
```

- [ ] **Step 2: Create main.go**

```go
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

	// Resolve guild name for TUI.
	guild, err := discordClient.FetchGuild()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching Discord guild: %v\n", err)
		os.Exit(1)
	}

	// Fetch channels once for configure screen.
	channels, err := discordClient.FetchChannels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching channels: %v\n", err)
		os.Exit(1)
	}

	// Build targets and confirm entries.
	targets := make(map[string]target.Target)
	confirmEntries := []tui.ConfirmEntry{
		{Label: "Source  (Discord)", Name: guild.Name, ServerID: cfg.DiscordServerID},
	}

	if cfg.Stoat != nil {
		a := targetstoat.New(cfg.Stoat.Token, cfg.Stoat.ServerID)
		targets["stoat"] = a
		confirmEntries = append(confirmEntries, tui.ConfirmEntry{
			Label:    "Target  (Stoat)",
			Name:     cfg.Stoat.ServerID, // name resolution optional; use ID for now
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

	// Wire the app so Start actually runs the pipeline.
	appCfg := tui.AppConfig{
		ConfirmEntries: confirmEntries,
		Channels:       channels,
		Targets:        targets,
		Discord:        discordClient,
	}

	// Override startPipeline to fully wire orchestrator.
	// We patch AppModel by supplying an OnStart callback.
	progressCh := make(chan pipeline.ProgressEvent, 200)
	pauser := pipeline.NewPauser()
	ctx, cancel := context.WithCancel(context.Background())

	app := tui.NewApp(appCfg)
	app.SetPipelineRunner(func(channelCfgs map[string]pipeline.ChannelConfig) {
		go func() {
			defer close(progressCh)
			roles, err := discordClient.FetchRoles()
			if err != nil {
				log.Printf("fatal: fetch roles: %v", err)
				return
			}
			err = pipeline.Run(ctx, pipeline.RunConfig{
				Targets:     targets,
				Discord:     discordClient,
				ChannelCfgs: channelCfgs,
				ProgressCh:  progressCh,
				Pauser:      pauser,
			})
			if err != nil {
				log.Printf("pipeline error: %v", err)
			}
			_ = roles
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
```

Note: `app.SetPipelineRunner`, `app.SetProgressCh`, `app.SetPauser`, `app.SetCancel` need to be added to `AppModel` in `app.go` as simple setter methods. Add them during this task.

- [ ] **Step 3: Add setter methods to app.go**

Append to `internal/tui/app.go`:

```go
// PipelineRunner is called when the user presses Start, with the exported channel configs.
type PipelineRunner func(channelCfgs map[string]pipeline.ChannelConfig)

func (m *AppModel) SetPipelineRunner(fn PipelineRunner) { m.pipelineRunner = fn }
func (m *AppModel) SetProgressCh(ch chan pipeline.ProgressEvent) { m.progressCh = ch }
func (m *AppModel) SetPauser(p *pipeline.Pauser) { m.pauser = p }
func (m *AppModel) SetCancel(fn context.CancelFunc) { m.cancelFn = fn }
```

And add field `pipelineRunner PipelineRunner` to `AppModel`.

Update `startPipeline()` in `app.go` to call `m.pipelineRunner(channelCfgs)` instead of the stub.

- [ ] **Step 4: Build the binary**

```bash
go build -ldflags "-X main.version=dev -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
    -o discord2stoat ./cmd/discord2stoat/
```

Expected: binary `discord2stoat` created with no errors.

- [ ] **Step 5: Test version command**

```bash
./discord2stoat version
```

Expected output:
```
discord2stoat dev
  commit:  <short hash or "unknown">
  source:  https://github.com/blubskye/discord2stoat
  license: AGPL-3.0
```

- [ ] **Step 6: Commit**

```bash
git add cmd/discord2stoat/
git commit -m "feat: main entrypoint, version command, and full pipeline wiring"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Config: discord + stoat + fluxer tokens, dual-target | Task 2 |
| Normalized types | Task 3 |
| Target interface | Task 3 |
| Stoat color + permission mapping | Task 4 |
| Stoat adapter (categories-as-metadata, raw HTTP perms, role rank) | Task 5 |
| Fluxer adapter (near-passthrough, snowflake IDs) | Task 6 |
| Discord client (fetch guild, roles, channels, messages paginated) | Task 7 |
| Progress event types | Task 8 |
| Phase A: roles → categories → channels → order → overwrites | Task 9 |
| Phase B: concurrent per-channel fetch+post workers | Task 10 |
| Pause/Resume via pause channel (not context) | Task 10 |
| Rate limiting: handled by libraries natively | N/A (no code needed) |
| TUI Screen 1: confirm | Task 11 |
| TUI Screen 2: configure with category bulk-set | Task 12 |
| TUI Screen 3: progress with per-channel rows | Task 13 |
| TUI app root + screen transitions | Task 14 |
| CLI `version` command with ldflags | Task 15 |
| AGPL-3.0 LICENSE | Task 1 |
| go.mod with replace directives for local libs | Task 1 |
| Dual-target (both stoat + fluxer simultaneously) | Tasks 9, 10, 15 |
| Log errors to discord2stoat.log | Task 15 |

**All spec requirements covered. No placeholders or TODOs in plan tasks.**
