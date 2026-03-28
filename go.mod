module github.com/blubskye/discord2stoat

go 1.26

replace (
	github.com/fluxergo/fluxergo => ./fluxergo
	github.com/sentinelb51/revoltgo => ./revoltgo
)

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/bwmarrin/discordgo v0.29.0
	github.com/disgoorg/omit v1.0.0
	github.com/disgoorg/snowflake/v2 v2.0.3
	github.com/fluxergo/fluxergo v0.0.0-00010101000000-000000000000
	github.com/sentinelb51/revoltgo v0.0.0-20260325033615-9efbbbc77577
)

require (
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/lxzan/gws v1.9.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/sasha-s/go-csync v0.0.0-20240107134140-fcbab37b09ad // indirect
	github.com/tinylib/msgp v1.6.3 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)
