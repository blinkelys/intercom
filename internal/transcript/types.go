package transcript

import "time"

const (
	// EventTypeUpdated is published when transcript state changes.
	EventTypeUpdated = "transcript.updated"
)

// Entry represents one finalized transcript message in the in-memory timeline.
type Entry struct {
	ID          string
	ChannelID   string
	ChannelName string
	Timestamp   time.Time
	Text        string
	Keywords    []string
	Highlights  []Highlight
}

// Highlight marks a matched keyword range.
type Highlight struct {
	Phrase string
	Color  string
	Start  int
	End    int
}

// Partial represents the latest temporary transcript for one channel.
type Partial struct {
	ChannelID   string
	ChannelName string
	Timestamp   time.Time
	Text        string
}

// Snapshot represents the current transcript state.
type Snapshot struct {
	Entries  []Entry
	Partials map[string]Partial
}

// Update is the lightweight event payload published when transcript state changes.
type Update struct {
	Kind      string
	ChannelID string
	EntryID   string
	Timestamp time.Time
}
