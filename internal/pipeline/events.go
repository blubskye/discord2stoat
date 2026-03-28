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
