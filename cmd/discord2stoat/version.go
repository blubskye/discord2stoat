// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// These variables are set at build time via:
//   go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc1234"
var (
	version = "dev"
	commit  = "unknown"
)
