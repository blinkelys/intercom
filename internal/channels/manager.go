package channels

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"procom/internal/config"
	"procom/internal/events"
)

// Manager owns mutable in-memory channel state.
type Manager struct {
	bus    *events.Bus
	logger *log.Logger

	mu       sync.RWMutex
	channels []Channel
	byID     map[string]int
	started  bool
}

// NewManager constructs a channel manager from runtime config.
func NewManager(cfg config.Config, bus *events.Bus, logger *log.Logger) (*Manager, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	channels := make([]Channel, 0, len(cfg.Channels))
	byID := make(map[string]int, len(cfg.Channels))
	for index, channel := range cfg.Channels {
		current := fromConfig(channel)
		if err := validateChannel(current); err != nil {
			return nil, fmt.Errorf("invalid channel %q: %w", current.ID, err)
		}
		channels = append(channels, current)
		byID[current.ID] = index
	}

	return &Manager{
		bus:      bus,
		logger:   logger,
		channels: channels,
		byID:     byID,
	}, nil
}

// Name returns the lifecycle component name.
func (m *Manager) Name() string {
	return "channels-manager"
}

// Start marks the manager active.
func (m *Manager) Start(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return fmt.Errorf("channels manager already started")
	}
	m.started = true
	return nil
}

// Stop marks the manager inactive.
func (m *Manager) Stop(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = false
	return nil
}

// Snapshot returns the current ordered channel set.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]Channel, len(m.channels))
	copy(channels, m.channels)
	return Snapshot{Channels: channels}
}

// Update mutates one channel in memory and publishes a change event.
func (m *Manager) Update(request UpdateRequest) (Channel, error) {
	updated := Channel{
		ID:          strings.TrimSpace(request.ID),
		Name:        strings.TrimSpace(request.Name),
		Color:       strings.TrimSpace(request.Color),
		Icon:        request.Icon,
		InputDevice: strings.TrimSpace(request.InputDevice),
		Language:    strings.TrimSpace(request.Language),
		Enabled:     request.Enabled,
	}
	if err := validateChannel(updated); err != nil {
		return Channel{}, err
	}

	m.mu.Lock()
	index, ok := m.byID[updated.ID]
	if !ok {
		m.mu.Unlock()
		return Channel{}, fmt.Errorf("unknown channel %q", updated.ID)
	}
	m.channels[index] = updated
	m.mu.Unlock()

	m.bus.Publish(events.Event{Type: EventTypeUpdated, Payload: Update{Channel: updated, OccurredAt: time.Now().UTC()}})
	return updated, nil
}

func fromConfig(channel config.ChannelConfig) Channel {
	return Channel{
		ID:          channel.ID,
		Name:        channel.Name,
		Color:       channel.Color,
		Icon:        channel.Icon,
		InputDevice: channel.InputDevice,
		Language:    channel.Language,
		Enabled:     channel.Enabled,
	}
}

func validateChannel(channel Channel) error {
	if channel.ID == "" {
		return fmt.Errorf("channel id is required")
	}
	if channel.Name == "" {
		return fmt.Errorf("channel name is required")
	}
	if channel.Color == "" {
		return fmt.Errorf("channel color is required")
	}
	if strings.TrimSpace(channel.Icon) == "" {
		return fmt.Errorf("channel icon is required")
	}
	if channel.Language == "" {
		return fmt.Errorf("channel language is required")
	}
	return nil
}
