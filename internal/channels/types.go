package channels

import "time"

const (
	// EventTypeUpdated is published when channel state changes.
	EventTypeUpdated = "channels.updated"
)

// Channel is the runtime read model for one communication channel.
type Channel struct {
	ID          string
	Name        string
	Color       string
	Icon        string
	InputDevice string
	Language    string
	Enabled     bool
}

// UpdateRequest contains mutable channel fields.
type UpdateRequest struct {
	ID          string
	Name        string
	Color       string
	Icon        string
	InputDevice string
	Language    string
	Enabled     bool
}

// Snapshot returns the current ordered channel state.
type Snapshot struct {
	Channels []Channel
}

// Update is the event payload emitted on channel changes.
type Update struct {
	Channel    Channel
	OccurredAt time.Time
}
