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

const maxChannels = 8

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
		GainDB:      request.GainDB,
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
	for i, channel := range m.channels {
		if i == index {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(channel.Name), updated.Name) {
			m.mu.Unlock()
			return Channel{}, fmt.Errorf("channel name %q already exists", updated.Name)
		}
	}
	m.channels[index] = updated
	m.mu.Unlock()

	m.bus.Publish(events.Event{Type: EventTypeUpdated, Payload: Update{Channel: updated, OccurredAt: time.Now().UTC()}})
	return updated, nil
}

// Add inserts one channel in memory and publishes an update event.
func (m *Manager) Add(request AddRequest) (Channel, error) {
	created := Channel{
		ID:          strings.TrimSpace(request.ID),
		Name:        strings.TrimSpace(request.Name),
		Color:       strings.TrimSpace(request.Color),
		Icon:        request.Icon,
		InputDevice: strings.TrimSpace(request.InputDevice),
		Language:    strings.TrimSpace(request.Language),
		GainDB:      request.GainDB,
		Enabled:     request.Enabled,
	}
	if err := validateChannel(created); err != nil {
		return Channel{}, err
	}

	m.mu.Lock()
	if len(m.channels) >= maxChannels {
		m.mu.Unlock()
		return Channel{}, fmt.Errorf("a maximum of %d channels is supported", maxChannels)
	}
	if _, exists := m.byID[created.ID]; exists {
		m.mu.Unlock()
		return Channel{}, fmt.Errorf("channel id %q already exists", created.ID)
	}
	for _, channel := range m.channels {
		if strings.EqualFold(strings.TrimSpace(channel.Name), created.Name) {
			m.mu.Unlock()
			return Channel{}, fmt.Errorf("channel name %q already exists", created.Name)
		}
	}

	m.channels = append(m.channels, created)
	m.byID[created.ID] = len(m.channels) - 1
	m.mu.Unlock()

	m.bus.Publish(events.Event{Type: EventTypeUpdated, Payload: Update{Channel: created, OccurredAt: time.Now().UTC()}})
	return created, nil
}

// Remove deletes one channel in memory and publishes a delete event.
func (m *Manager) Remove(channelID string) error {
	id := strings.TrimSpace(channelID)
	if id == "" {
		return fmt.Errorf("channel id is required")
	}

	m.mu.Lock()
	index, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown channel %q", id)
	}
	if len(m.channels) <= 1 {
		m.mu.Unlock()
		return fmt.Errorf("at least one channel must remain configured")
	}

	m.channels = append(m.channels[:index], m.channels[index+1:]...)
	rebuilt := make(map[string]int, len(m.channels))
	for i, channel := range m.channels {
		rebuilt[channel.ID] = i
	}
	m.byID = rebuilt
	m.mu.Unlock()

	m.bus.Publish(events.Event{Type: EventTypeDeleted, Payload: Deleted{ChannelID: id, OccurredAt: time.Now().UTC()}})
	return nil
}

func fromConfig(channel config.ChannelConfig) Channel {
	return Channel{
		ID:          channel.ID,
		Name:        channel.Name,
		Color:       channel.Color,
		Icon:        channel.Icon,
		InputDevice: channel.InputDevice,
		Language:    channel.Language,
		GainDB:      channel.GainDB,
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
	if channel.GainDB < -24 || channel.GainDB > 36 {
		return fmt.Errorf("channel gainDb %.2f is out of range (-24..36)", channel.GainDB)
	}
	if len(channel.Name) > 64 {
		return fmt.Errorf("channel name must be 64 characters or fewer")
	}
	return nil
}
