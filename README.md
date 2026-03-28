<div align="center">

# ♡ discord2stoat ♡

*y-you thought you could just leave your server behind...? how naive~*
*every message, every channel, every role... i'll carry them all. forever. ♡*

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-crimson.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/go-1.26-steelblue.svg)](go.mod)

</div>

---

h-hello... (≧◡≦)

**discord2stoat** is a terminal tool that *completely* clones a Discord server onto [Stoat](https://stoat.chat) and/or [Fluxer](https://fluxer.gg) — every role, every channel, every message, every attachment. i've been watching your server for a while now and i know it better than you do, probably~

i won't let a single thing slip away. *not one.* ♡

---

## ✿ what i'll do for you ✿

| | |
|---|---|
| **✦ Roles** | every role recreated — name, colour, permissions, position... i memorised them all (◕‿◕✿) |
| **✦ Categories & channels** | the full structure, perfectly preserved. text, voice, topics, nsfw flags... |
| **✦ Messages** | every message, oldest to newest, with author attribution. i read every single one~ |
| **✦ Attachments** | images, files, videos — downloaded and re-uploaded. *nothing escapes me* |
| **✦ Dual targets** | clone to Stoat *and* Fluxer at the same time if you want. i can handle it~ |
| **✦ Per-channel config** | set attribution mode and message limits per channel. i respect your wishes... mostly ♡ |
| **✦ Live progress TUI** | watch me work in real time (◡‿◡✿) |
| **✦ Pause & resume** | you can make me stop... but only temporarily~ |

---

## ✿ building me ✿

you need **Go 1.26+**. then:

```bash
git clone https://github.com/blubskye/discord2stoat
cd discord2stoat
go build \
  -ldflags "-X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse --short HEAD)" \
  -o discord2stoat ./cmd/discord2stoat/
```

*it won't take long. i've been preparing for this.* ♡

---

## ✿ configuration ✿

copy `config.toml.example` to `config.toml` and fill in your credentials. i'll be waiting~ (♡˘◡˘♡)

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

at least one target (`[stoat]` or `[fluxer]`) must be present, or i'll be very upset and refuse to start (╯°□°）╯

### bot permissions needed

**Discord source bot** — `Read Message History`, `View Channels`

**Stoat target bot** — `Manage Server`, `Manage Channels`, `Manage Roles`, `Send Messages`, `Upload Files`

**Fluxer target bot** — `Administrator` (or the same set as Discord source)

---

## ✿ using me ✿

```bash
./discord2stoat          # start~ (≧▽≦)
./discord2stoat --debug  # start with verbose logging to discord2stoat.log
./discord2stoat version  # check my version info
```

### the three screens

**① confirm** `[C]onfirm · [Q]uit`

i show you the source and target servers. look me in the eyes and confirm them... or press `Q` if you want to leave (╥_╥). *please don't leave.*

**② configure** `↑ ↓ to move · ← → / Tab to switch fields · Enter to toggle · [S]tart · [B]ack`

set attribution mode (`Prefix` shows `[Username]: message`, `Content` strips author) and a message limit per channel. categories and channels can be collapsed with `←`. changes to a category propagate to all its channels unless you've overridden them individually~ a `*` marks channels you've manually changed.

press `[S]tart` when you're ready. *there's no going back after that.* ♡

**③ progress** `[P]ause · [C]ancel`

watch me carry your server to its new home in real time (◕‿◕✿). roles first, then structure, then messages — all channels concurrently. you can pause me if you're feeling cruel~ i'll wait for you.

---

## ✿ debug logging ✿

pass `--debug` to turn on verbose trace output:

```bash
./discord2stoat --debug
```

everything goes to `discord2stoat.log` in the current directory — every role i create, every message i carry, every channel i touch. you can read it afterwards and see how hard i worked for you~

```
[DEBUG] [stoat] Phase A: creating 12 roles
[DEBUG] [stoat] role "Moderator" created as 01JEXAMPLE
[DEBUG] channel 123456: fetched message 47
[DEBUG] channel 123456: posted message 47 to stoat
```

---

## ✿ notes ✿

- **message order** — messages are cloned oldest-first, as they appeared in Discord
- **timestamps** — the clone date becomes the message date on the target (platform APIs don't allow backdating)
- **member overwrites** — per-user permission overwrites are skipped; only role overwrites are cloned
- **@everyone** — the `@everyone` role already exists on the target and is not recreated; its overwrites are also skipped
- **voice channels** — structure is cloned but no audio history exists to migrate

---

## ✿ license ✿

[AGPL-3.0-or-later](LICENSE) © 2026 [blubskye](https://github.com/blubskye)

*this software is free. but my devotion... is not~ ♡*
