# Discord Server Clone Tool — Design Spec
_2026-03-27_

## Overview

A one-shot Go TUI tool that clones a Discord server (source) onto a Stoat (Revolt) server, a Fluxer server, or both simultaneously. Clones roles, channels, categories, channel ordering, NSFW flags, permission overwrites, and message content. Designed for large, long-running servers with full concurrent pipelines and API-driven rate limiting.

Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). Source: https://github.com/blubskye/discord2stoat

---

## Config

`config.toml` at the working directory:

```toml
discord_token     = "Bot TOKEN"
discord_server_id = "1234567890"

# Enable one or both targets by populating their sections.
# At least one must be present.

[stoat]
token     = "stoat-bot-token"
server_id = "abcdefgh"

[fluxer]
token     = "fluxer-bot-token"
server_id = "9876543210"
```

Both `[stoat]` and `[fluxer]` blocks can be present simultaneously. If both are populated, the tool clones to both platforms in parallel. A section is skipped if its token is empty.

---

## Project Structure

```
cmd/discord2stoat/
  main.go                 entry point: parse CLI args, load config, build clients, start TUI
  version.go              version string embedded at build time via ldflags

internal/
  config/
    config.go             parse config.toml

  discord/
    client.go             wraps discordgo; FetchGuild, FetchChannels, FetchRoles, FetchMessages
    types.go              re-exports discordgo types used across packages

  normalized/
    types.go              NormalizedRole, NormalizedChannel, NormalizedOverwrite,
                          NormalizedMessage, NormalizedChannelOrder

  target/
    target.go             Target interface
    stoat/
      adapter.go          implements Target via revoltgo
      permissions.go      Discord permission bits → Revolt permission bits
      color.go            Discord int color → CSS hex string
    fluxer/
      adapter.go          implements Target via fluxergo (near-passthrough)

  pipeline/
    orchestrator.go       phase coordination; owns worker pools and progress channel
    structure.go          Phase A: roles → categories → channels → positions → overwrites
    messages.go           Phase B: per-channel fetch+post goroutine pairs

  tui/
    app.go                bubbletea root model; owns screen transitions
    confirm.go            Screen 1: confirm source + dest server names
    configure.go          Screen 2: per-channel/category attribution + message limits
    progress.go           Screen 3: live progress rows per channel + category aggregates
    events.go             ProgressEvent types sent from workers to TUI

revoltgo/                 cloned dependency
fluxergo/                 cloned dependency
docs/
LICENSE                   GNU AGPL-3.0
config.toml
go.mod
go.sum
```

---

## CLI

The binary accepts a `version` subcommand:

```
discord2stoat version
```

Output:
```
discord2stoat dev
  commit:  a1b2c3d
  source:  https://github.com/blubskye/discord2stoat
  license: AGPL-3.0
```

Version, commit hash, and build date are embedded at build time via `-ldflags`:

```
go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse --short HEAD)" ./cmd/discord2stoat
```

When built without ldflags (e.g. `go run`), version shows as `dev` and commit as `unknown`.

Running the binary with no arguments (or any argument other than `version`) starts the TUI.

---

## Dual-Target Support

When both `[stoat]` and `[fluxer]` are configured, the orchestrator builds two `Target` instances. Phase A and Phase B run against both targets. Each target has its own independent worker pools and rate limiters — they do not share state.

The TUI confirm screen lists both destinations:

```
  Source  (Discord):  My Big Discord Server  [1234567890]
  Target  (Stoat):    My Stoat Server        [abcdefgh]
  Target  (Fluxer):   My Fluxer Server       [9876543210]

  [Confirm]  [Quit]
```

The progress screen shows a per-target section for each channel row.

---

## Target Interface

```go
// internal/target/target.go

type Target interface {
    // Phase A
    CreateRole(r normalized.Role) (newID string, err error)
    SetRoleOrder(roles []normalized.RoleOrder) error
    CreateChannel(c normalized.Channel) (newID string, err error)
    SetChannelOrder(updates []normalized.ChannelOrder) error
    SetChannelPermissions(channelID string, overwrites []normalized.Overwrite) error

    // Phase B
    SendMessage(channelID string, msg normalized.Message) error
}
```

---

## Normalized Types

```go
// internal/normalized/types.go

type Role struct {
    Name        string
    Color       int    // Discord int; adapters convert as needed
    Permissions int64  // Discord permission bits; adapters remap
    Hoist       bool
    Position    int    // Discord position; adapters map to target ordering
}

type Channel struct {
    Name        string
    Type        ChannelType  // Text, Voice, Category
    Topic       string
    NSFW        bool
    Position    int
    ParentID    string       // original Discord category ID; remapped by orchestrator
    Overwrites  []Overwrite
}

type ChannelOrder struct {
    ChannelID string  // new target ID
    Position  int
    ParentID  string  // new target parent ID
}

type RoleOrder struct {
    RoleID   string  // new target role ID
    Position int     // Discord position; adapters invert to rank as needed
}

type Overwrite struct {
    RoleID string  // new target role ID (remapped by orchestrator)
    Allow  int64
    Deny   int64
}

type Message struct {
    Content     string
    AuthorName  string       // used when attribution mode is Prefix
    Timestamp   time.Time
    Attachments []Attachment // downloaded from Discord CDN; re-uploaded by each target adapter
}

type Attachment struct {
    Filename string
    Data     []byte // raw file bytes downloaded from Discord CDN
}

type ChannelType int
const (
    ChannelTypeText ChannelType = iota
    ChannelTypeVoice
    ChannelTypeCategory
)
```

---

## Platform Adapters

### Stoat Adapter (`internal/target/stoat/adapter.go`)

| Concern | Mapping |
|---|---|
| Categories | Not a channel type; built as `ServerCategory` entries in `ServerEditData.Categories` after all channels are created |
| Channel order | Category's `Channels []string` array order determines position |
| Role rank | Discord `position` → Revolt `rank` (inverted: lower rank = higher in list) |
| Role color | `int` (e.g. `0xFF5733`) → CSS hex string `"#FF5733"` |
| Permissions | Discord bit flags → Revolt bit flags via lookup table in `permissions.go` |
| Role permissions | `ServersRoleCreate` (name+rank) then `ServerRoleEdit` (colour+hoist); server-level permission bits set via direct HTTP PATCH since `ServerRoleEditData` lacks a `Permissions` field |
| Channel overwrites | `ChannelPermissionsSet(cID, rID, PermissionOverwrite{Allow, Deny})` |
| Messages | Upload each `Attachment.Data` via `session.HTTP.Request(POST, EndpointAutumn("attachments"), &File{Name, Reader}, &FileAttachment)` → collect IDs → `ChannelMessageSend(cID, MessageSend{Content: formatted, Attachments: []string{ids...}})` |

### Fluxer Adapter (`internal/target/fluxer/adapter.go`)

Near-passthrough — Fluxer uses the same permission bit model and snowflake IDs as Discord.

| Concern | Mapping |
|---|---|
| Categories | `GuildCategoryChannelCreate` — first-class channel type |
| Channel order | `UpdateChannelPositions(guildID, []GuildChannelPositionUpdate)` |
| Role positions | `UpdateRolePositions(guildID, []RolePositionUpdate)` |
| Role creation | `CreateRole(guildID, RoleCreate{Name, Color, Permissions, Hoist})` |
| Channel overwrites | `UpdatePermissionOverwrite(channelID, roleID, PermissionOverwriteUpdate)` |
| Messages | For each `Attachment.Data`, call `MessageCreate.WithFiles(fluxer.NewFile(filename, "", bytes.NewReader(data)))` → `CreateMessage(channelID, messageCreate)` |

---

## Pipeline Phases

### Phase A — Structure (sequential within dependency order)

```
1. Fetch: Discord roles, channels (with overwrites), categories
2. Create roles on target
   → build discordRoleID → targetRoleID map
3. Create categories on target
   → build discordCategoryID → targetCategoryID map
4. Create channels (text + voice), parent IDs remapped via step 3
   → build discordChannelID → targetChannelID map
5. Set channel positions / role ranks (single batch call per platform)
6. Set channel permission overwrites (role IDs remapped via step 2)
7. (Stoat only) Patch server categories metadata to embed channel order
```

Steps must run in order. Small goroutine pool (3–5 workers) handles step 4 for throughput while keeping creation order deterministic.

### Phase B — Messages (fully concurrent, starts after Phase A)

One goroutine pair per channel:

```
Fetch goroutine                          Post goroutine
─────────────────                        ──────────────────
discordgo.ChannelMessages                reads from buffer
  (paginate oldest→newest,               formats message per
   100 per page, cursor-based)           attribution setting
         │                               sends via Target.SendMessage
         ▼                                      │
  buffered chan (size 500)  ──────────►         │
                                         emits ProgressEvent
```

- Both `discordgo` and `revoltgo`/`fluxergo` handle rate limiting internally from response headers — workers call freely and the library blocks when the window is exhausted
- Context cancellation (from TUI Pause/Cancel) propagates to all goroutines via a shared `context.Context`
- Fetch goroutines respect the per-channel message limit configured in the TUI (`All` or `Last N`)
- For each message, attachments (images, videos, files) are downloaded from the Discord CDN URL into `[]byte` once, then included in the normalized message so both target adapters can re-upload independently. Attachments larger than 100 MB are skipped and logged.
- When a channel has no messages (voice channels, empty text channels), the fetch goroutine exits immediately; no post goroutine is spawned

---

## TUI Screens

### Screen 1 — Confirm

Displays the resolved names of the source Discord server and destination Stoat/Fluxer server fetched live from both APIs. User confirms before any writes happen.

```
  discord2stoat

  Source  (Discord):  My Big Discord Server  [1234567890]
  Target  (Stoat):    My Stoat Server        [abcdefgh]

  [Confirm]  [Quit]
```

### Screen 2 — Configure

Scrollable list of all channels grouped by category. Category rows act as bulk setters for all uncustomized children. Individual channel rows can override the category setting; overridden channels show a dim `*`.

Navigation: arrow keys to move, enter to toggle dropdowns, left/right to collapse/expand categories.

```
  [Select All]  Attribution: [Prefix ▾]  Messages: [All ▾]

  ▼ General          Attribution: [Prefix  ▾]  Messages: [All    ▾]
      #general       Attribution: [Prefix  ▾]  Messages: [All    ▾]
      #announcements Attribution: [Prefix  ▾]  Messages: [Last N: 500 ] *

  ▼ Community        Attribution: [Content ▾]  Messages: [All    ▾]
      #off-topic     Attribution: [Content ▾]  Messages: [All    ▾]
      #memes         Attribution: [Content ▾]  Messages: [All    ▾]

  Uncategorized      Attribution: [Prefix  ▾]  Messages: [All    ▾]
      #welcome       Attribution: [Prefix  ▾]  Messages: [All    ▾]

  [Start]  [Back]
```

### Screen 3 — Progress

Live-updating view. Workers emit `ProgressEvent` values onto a shared channel; the bubbletea model consumes them on each `Update` tick without blocking workers.

```
  Roles      ████████░░   8 / 10
  Structure  ██████████   done

  ▼ General                        4,902 / 11,000 msgs posted
      #general        fetch 10000/10000  post 9,841/10000  ✓
      #announcements  fetch  500/500     post   500/500    ✓

  ▼ Community                      2,100 / ∞ msgs posted
      #off-topic      fetch  891/∞       post   750/∞      …
      #memes          fetch  203/∞       post   180/∞      …

  voice-1   ✓
  voice-2   ✓

  [Pause]  [Cancel]
```

Pause sends a signal on a shared pause channel; each worker checks this channel between operations and blocks until Resume sends a resume signal. Cancel closes the root context, causing all workers to drain and exit. Context cancellation is one-way so pause/resume uses a separate `chan struct{}` pair rather than context.

---

## Attribution Formats

| Mode | Output |
|---|---|
| Prefix | `[Username]: message content` |
| Content only | `message content` |

---

## Error Handling

- Per-channel errors are shown inline in the progress view and logged to `discord2stoat.log`
- A failed channel does not stop other channels
- Phase A errors (structure creation) are fatal — tool exits with a clear message before any messages are sent
- On cancel/interrupt, already-posted messages are not rolled back (no delete-on-cancel)

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/sentinelb51/revoltgo` | Stoat API client (local) |
| `github.com/fluxergo/fluxergo` | Fluxer API client (local) |
| `github.com/bwmarrin/discordgo` | Discord API client |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/BurntSushi/toml` | Config parsing |
