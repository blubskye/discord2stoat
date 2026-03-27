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
