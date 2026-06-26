package transcript

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/keywords"
	"procom/internal/speech"
)

// Manager owns in-memory transcript ordering and per-channel partial state.
type Manager struct {
	bus      *events.Bus
	logger   *log.Logger
	channels map[string]config.ChannelConfig
	matcher  *keywords.Matcher

	mu           sync.RWMutex
	entries      []Entry
	partials     map[string]Partial
	nextSequence uint64
	subscription *events.Subscription
	started      bool
	cancel       context.CancelFunc
	workerDone   chan struct{}
}

// NewManager constructs a transcript manager from runtime dependencies.
func NewManager(cfg config.Config, bus *events.Bus, logger *log.Logger) (*Manager, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	channels := make(map[string]config.ChannelConfig, len(cfg.Channels))
	for _, channel := range cfg.Channels {
		channels[channel.ID] = channel
	}

	return &Manager{
		bus:      bus,
		logger:   logger,
		channels: channels,
		matcher:  keywords.NewMatcher(cfg.Keywords),
		partials: make(map[string]Partial),
	}, nil
}

// Name returns the lifecycle component name.
func (m *Manager) Name() string {
	return "transcript-manager"
}

// Start subscribes to speech events and begins maintaining transcript state.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("transcript manager already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	subscription, err := m.bus.Subscribe()
	if err != nil {
		cancel()
		m.mu.Unlock()
		return fmt.Errorf("subscribe to event bus: %w", err)
	}
	m.cancel = cancel
	m.subscription = subscription
	m.workerDone = make(chan struct{})
	m.started = true
	m.mu.Unlock()

	go m.consume(runCtx, subscription)
	return nil
}

// Stop detaches the transcript manager from the runtime.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	cancel := m.cancel
	subscription := m.subscription
	workerDone := m.workerDone
	m.cancel = nil
	m.subscription = nil
	m.workerDone = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if subscription != nil {
		subscription.Close()
	}

	if workerDone == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-workerDone:
		return nil
	}
}

// Snapshot returns a copy of the current transcript timeline and partials.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]Entry, len(m.entries))
	copy(entries, m.entries)

	partials := make(map[string]Partial, len(m.partials))
	for channelID, partial := range m.partials {
		partials[channelID] = partial
	}

	return Snapshot{
		Entries:  entries,
		Partials: partials,
	}
}

// Entries returns a copy of the finalized transcript entries.
func (m *Manager) Entries() []Entry {
	return m.Snapshot().Entries
}

// Entry returns one finalized transcript entry by ID.
func (m *Manager) Entry(id string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

// Partials returns a copy of the current per-channel partial transcript state.
func (m *Manager) Partials() map[string]Partial {
	return m.Snapshot().Partials
}

// UpdateKeywords replaces keyword matching rules at runtime.
func (m *Manager) UpdateKeywords(rules []config.KeywordConfig) {
	m.mu.Lock()
	m.matcher = keywords.NewMatcher(rules)
	m.mu.Unlock()
}

func (m *Manager) consume(ctx context.Context, subscription *events.Subscription) {
	defer close(m.workerDone)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			switch event.Type {
			case speech.EventTypeResultPartial:
				result, ok := event.Payload.(speech.Result)
				if !ok {
					m.logger.Printf("transcript partial ignored: unexpected payload type %T", event.Payload)
					continue
				}
				m.applyPartial(result)
			case speech.EventTypeResultFinal:
				result, ok := event.Payload.(speech.Result)
				if !ok {
					m.logger.Printf("transcript final ignored: unexpected payload type %T", event.Payload)
					continue
				}
				m.applyFinal(result)
			case channels.EventTypeUpdated:
				update, ok := event.Payload.(channels.Update)
				if !ok {
					continue
				}
				m.mu.Lock()
				m.channels[update.Channel.ID] = config.ChannelConfig{
					ID:          update.Channel.ID,
					Name:        update.Channel.Name,
					Color:       update.Channel.Color,
					Icon:        update.Channel.Icon,
					InputDevice: update.Channel.InputDevice,
					Language:    update.Channel.Language,
					Enabled:     update.Channel.Enabled,
				}
				m.mu.Unlock()
			case channels.EventTypeDeleted:
				deleted, ok := event.Payload.(channels.Deleted)
				if !ok {
					continue
				}
				m.mu.Lock()
				delete(m.channels, deleted.ChannelID)
				delete(m.partials, deleted.ChannelID)
				m.mu.Unlock()
			}
		}
	}
}

func (m *Manager) applyPartial(result speech.Result) {
	timestamp := result.ReceivedAt
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	if strings.TrimSpace(result.Text) == "" {
		m.mu.Lock()
		delete(m.partials, result.ChannelID)
		m.mu.Unlock()
		m.publishUpdate("partial", result.ChannelID, "", timestamp)
		return
	}

	partial := Partial{
		ChannelID:   result.ChannelID,
		ChannelName: m.channelName(result.ChannelID),
		Timestamp:   timestamp,
		Text:        result.Text,
	}

	m.mu.Lock()
	m.partials[result.ChannelID] = partial
	m.mu.Unlock()

	m.publishUpdate("partial", result.ChannelID, "", timestamp)
}

func (m *Manager) applyFinal(result speech.Result) {
	timestamp := result.ReceivedAt
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	entry := Entry{
		ID:          m.nextID(),
		ChannelID:   result.ChannelID,
		ChannelName: m.channelName(result.ChannelID),
		Timestamp:   timestamp,
		Text:        result.Text,
	}
	entry.Keywords, entry.Highlights = m.matchKeywords(result.Text)

	m.mu.Lock()
	delete(m.partials, result.ChannelID)
	m.entries = append(m.entries, entry)
	sort.SliceStable(m.entries, func(left, right int) bool {
		if m.entries[left].Timestamp.Equal(m.entries[right].Timestamp) {
			return m.entries[left].ID < m.entries[right].ID
		}
		return m.entries[left].Timestamp.Before(m.entries[right].Timestamp)
	})
	m.mu.Unlock()

	m.publishUpdate("final", result.ChannelID, entry.ID, timestamp)
}

func (m *Manager) matchKeywords(text string) ([]string, []Highlight) {
	keywordsFound, matches := m.matcher.Match(text)
	if len(matches) == 0 {
		return keywordsFound, nil
	}

	highlights := make([]Highlight, 0, len(matches))
	for _, match := range matches {
		highlights = append(highlights, Highlight{
			Phrase: match.Phrase,
			Color:  match.Color,
			Start:  match.Start,
			End:    match.End,
		})
	}

	return keywordsFound, highlights
}

func (m *Manager) nextID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextSequence++
	return fmt.Sprintf("tx-%06d", m.nextSequence)
}

func (m *Manager) channelName(channelID string) string {
	channel, ok := m.channels[channelID]
	if !ok || channel.Name == "" {
		return channelID
	}
	return channel.Name
}

func (m *Manager) publishUpdate(kind string, channelID string, entryID string, timestamp time.Time) {
	m.bus.Publish(events.Event{Type: EventTypeUpdated, Payload: Update{
		Kind:      kind,
		ChannelID: channelID,
		EntryID:   entryID,
		Timestamp: timestamp,
	}})
}
