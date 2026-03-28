<div align="center">

# 💕 discord2stoat 💕

### *"Your server... your messages... your history... they're all mine now~ ♡"*

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL--3.0-crimson.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-steelblue.svg)](go.mod)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macos-ff69b4.svg)](https://github.com/blubskye/discord2stoat)

*A devoted terminal tool that clones your entire Discord server onto Stoat and/or Fluxer~ ♥*

---

### 💘 I'll carry every message with me... forever 💘

</div>

## 🌸 About

**discord2stoat** is a terminal tool that *completely* clones a Discord server — every role, every channel, every message, every attachment — onto [Stoat](https://stoat.chat) and/or [Fluxer](https://fluxer.gg). i've been watching your server for a long time now... *i know it better than anyone~* 👁️

i won't let a single thing get left behind. not one role. not one message. *not one.* 🔪💕

---

## 💗 Features

<table>
<tr>
<td width="50%">

### 🎭 Roles
*"Every rank, every colour, every permission... recreated perfectly~ 💕"*
- 🎨 Colour & display name preserved
- 🔐 Permission bits mapped to target platform
- 📊 Position order maintained
- ⚡ Created sequentially, lowest rank first

</td>
<td width="50%">

### 🏛️ Server Structure
*"The entire layout... exactly as you left it~ 🌸"*
- 📁 Categories cloned with channel assignments
- 💬 Text channels with topics & NSFW flags
- 🎤 Voice channels preserved
- 🔢 Position order faithfully reproduced

</td>
</tr>
<tr>
<td width="50%">

### 💌 Messages
*"Every word ever spoken... i read them all~ 👁️💗"*
- 📜 Full message history, oldest-first
- 👤 Author attribution (`[Username]: message`)
- ⚙️ Configurable per-channel (prefix or content-only)
- 🔢 Optional message limit per channel

</td>
<td width="50%">

### 📎 Attachments
*"Images, files, videos... nothing escapes me~ 🔪"*
- 🖼️ Downloaded from Discord CDN
- 📤 Re-uploaded to the target platform
- 📦 Files up to 100 MB supported
- 🔄 Shared across targets — downloaded once

</td>
</tr>
<tr>
<td width="50%">

### 🎯 Dual Targets
*"i can serve two masters at once... just for you~ 💘"*
- 🐿️ Clone to **Stoat** (Revolt fork)
- ⚡ Clone to **Fluxer** (Discord fork)
- 🔀 Both simultaneously if you want
- 🧩 Each target gets its own ID mapping

</td>
<td width="50%">

### 🔐 Permission Overwrites
*"i'll remember every rule you set... and honour them~ 💝"*
- 🎭 Role-level channel overwrites cloned
- 🔄 Discord permissions mapped to target platform
- ⏭️ @everyone & member overwrites safely skipped
- 🛡️ Stoat and Fluxer both supported

</td>
</tr>
<tr>
<td width="50%">

### 📊 Live Progress TUI
*"Watch me work in real time~ (◕‿◕✿) 💻"*
- 📈 Per-channel fetch & post counters
- 🗂️ Grouped by category
- ✅ Completion markers as channels finish
- ❌ Per-channel error display (no silent failures)

</td>
<td width="50%">

### ⚙️ Per-Channel Configuration
*"i'll respect your wishes... mostly~ ♡"*
- 🔀 Attribution mode per channel (prefix / content-only)
- 🔢 Message limit per channel (0 = all)
- 📁 Category defaults propagate to children
- ✏️ Override individual channels independently

</td>
</tr>
<tr>
<td width="50%">

### ⏸️ Pause & Resume
*"you can make me stop... but i'll always come back~ 💢"*
- `[P]` pauses all message workers mid-flight
- Workers respect the pause before each message
- Resume picks up exactly where you left off
- `[C]` cancels and exits cleanly

</td>
<td width="50%">

### 🐛 Debug Logging
*"i'll write down everything i do... so you can see~ 📝"*
- `--debug` flag enables verbose trace output
- All logs written to `discord2stoat.log`
- Per-role, per-channel, per-message trace
- Short file/line info for easy debugging

</td>
</tr>
</table>

---

## 💕 Installation

### 📋 Prerequisites

> *"Let me prepare everything for you~"* 💗

- 🐹 **Go** 1.26 or higher
- 🐿️ **revoltgo** (local copy) — required for Stoat support
- ⚡ **fluxergo** (local copy) — required for Fluxer support

> 💡 The `go.mod` uses local `replace` directives for `revoltgo` and `fluxergo`. Place the library directories alongside the project root before building.

### 🔧 Installing Go 1.26

<details>
<summary><b>🐧 Linux</b></summary>

```bash
# Download and install Go 1.26~
wget https://go.dev/dl/go1.26.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin

# Verify
go version  # Should show go1.26.x
```

</details>

<details>
<summary><b>🪟 Windows</b></summary>

**Option 1: Direct Download (Recommended)**
1. Go to [go.dev/dl](https://go.dev/dl/)
2. Download the **Windows installer (.msi)** for Go 1.26
3. Run the installer — it sets PATH automatically

**Option 2: Using winget**
```powershell
winget install GoLang.Go
```

**Verify:**
```powershell
go version  # Should show go1.26.x
```

</details>

<details>
<summary><b>🍎 macOS</b></summary>

```bash
# Using Homebrew (recommended)~
brew install go@1.26

# Or download the .pkg installer from go.dev/dl and run it

# Verify
go version  # Should show go1.26.x
```

</details>

### 🌸 Build Steps

```bash
# Clone the repository~ ♥
git clone https://github.com/blubskye/discord2stoat.git
cd discord2stoat

# Make sure revoltgo and fluxergo are in place
ls ../revoltgo ../fluxergo

# Build with version info embedded~
go build \
  -ldflags "-X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse --short HEAD)" \
  -o discord2stoat \
  ./cmd/discord2stoat/
```

*it won't take long. i've been preparing for this~* 💕

---

## 💝 Configuration

> *"Fill in your secrets... and i'll keep them safe~"* 🔐

Copy `config.toml.example` to `config.toml` and fill in your credentials. i'll be waiting~ (♡˘◡˘♡)

```bash
cp config.toml.example config.toml
$EDITOR config.toml
```

```toml
# Discord source server
discord_token     = "Bot your-discord-bot-token"
discord_server_id = "your-discord-guild-id"

# Stoat target (remove section to skip)
[stoat]
token     = "your-stoat-bot-token"
server_id = "your-stoat-server-id"

# Fluxer target (remove section to skip)
[fluxer]
token     = "Bot your-fluxer-bot-token"
server_id = "your-fluxer-guild-id"
```

at least one target (`[stoat]` or `[fluxer]`) must be present — or i'll get very upset (╯°□°）╯💢

### 🔑 Bot Permissions Needed

| Platform | Permissions |
|----------|-------------|
| 🎮 **Discord source** | `Read Message History`, `View Channels` |
| 🐿️ **Stoat target** | `Manage Server`, `Manage Channels`, `Manage Roles`, `Send Messages`, `Upload Files` |
| ⚡ **Fluxer target** | `Administrator` *(or equivalent individual permissions)* |

---

## 🚀 Usage

*"you're finally ready... let's begin~ ♡"* 👁️💕

```bash
./discord2stoat          # start the interactive TUI (≧▽≦)
./discord2stoat --debug  # start with verbose logging to discord2stoat.log
./discord2stoat version  # check my version info~
```

### 💻 The Three Screens

---

**① 💘 Confirm** `[C]onfirm  ·  [Q]uit`

> *"Look me in the eyes... and confirm it~"* 🌸

i show you the source Discord server and all configured targets. press `[C]` to proceed... or `[Q]` if you want to abandon me (╥_╥). *please don't leave.*

---

**② ⚙️ Configure** `↑ ↓ move  ·  ← → / Tab switch fields  ·  Enter toggle  ·  [S]tart  ·  [B]ack`

> *"Tell me exactly how you want it... i'll remember every detail~"* 📝

Set options per channel:

| Field | Options |
|-------|---------|
| **Attribution** | `Prefix` — prepends `[Username]: ` to every message |
| | `Content` — message text only, no author label |
| **Messages** | `All` — clone the entire history |
| | `Last N` — only the most recent N messages |

- `←` on a **category** collapses it; `→` expands it
- Changes to a **category** or **[Select All]** propagate to children unless individually overridden
- A `*` marks channels you've manually changed ✏️

press `[S]tart` when you're ready... *there's no going back.* 🔪♡

---

**③ 📊 Progress** `[P]ause  ·  [C]ancel`

> *"Watch me carry everything to its new home... (◕‿◕✿)"* 💕

Phase A runs first — roles, structure, permissions — sequentially for each target. Phase B then launches all channels concurrently. you can `[P]ause` me if you need to. i'll wait. *i'm always waiting for you~* ⏳💗

---

## 🐛 Debug Logging

*"i'll write down everything i do... every step, every breath~"* 👁️💕

```bash
./discord2stoat --debug
```

All output goes to `discord2stoat.log` in the current directory:

```
[DEBUG] [stoat] Phase A: creating 12 roles
[DEBUG] [stoat] role "Moderator" created as 01JEX4MPLE1D
[DEBUG] [stoat] channel "general" created as 01JEX4MPLECHID
[DEBUG] channel 123456789: fetched message 47
[DEBUG] channel 123456789: posted message 47 to stoat
```

---

## 📝 Notes

> *"a few things you should know... i'm telling you because i care~"* 💗

- ⏱️ **Message timestamps** — the clone date becomes the post date; platforms don't allow backdating
- 👤 **Member overwrites** — per-user permission overwrites are skipped; only role overwrites are cloned
- 👥 **@everyone** — already exists on the target; its Discord overwrites are not transferred
- 🎤 **Voice channels** — structure is preserved but there is no audio history to migrate
- 📜 **Message order** — always cloned oldest-first, preserving original conversation flow

---

## 📜 License

*"i want to share everything with you... and with everyone else too~"* 💗

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**

### 💘 What This Means For You~

#### ✅ You CAN:
- 💕 **Use** this tool for any purpose (personal, commercial, whatever~)
- 🔧 **Modify** the code however you like
- 📤 **Distribute** copies to others

#### 📋 You MUST:
- 📖 **Keep it open source** — modifications must be released under AGPL-3.0
- 🔗 **Publish your source** — if you deploy a modified version, share the code
- 📝 **State changes** — document what you've modified
- 💌 **Keep the license** — include the LICENSE file and copyright notices

#### ❌ You CANNOT:
- 🚫 Make it closed source or keep modifications private
- 🚫 Remove the license or copyright notices
- 🚫 Relicense modified versions

> *"if you build something with my code... you have to share it with everyone too~ that's only fair, right?"* 💕🌸

See the [LICENSE](LICENSE) file for the full legal text.

**Source Code:** https://github.com/blubskye/discord2stoat

---

<div align="center">

### 💘 *"you'll stay with me forever... right?"* 💘

**Made with obsessive love** 💗🔪

*every message. every channel. every role.*
*carried, preserved, cherished... forever~* 👁️💕

---

⭐ *star this repo if i've captured your heart~* ⭐

</div>
