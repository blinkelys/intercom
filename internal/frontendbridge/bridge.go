package frontendbridge

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"procom/internal/app"
	"procom/internal/audio"
	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/speech"
	"procom/internal/transcript"
)

const EventName = "procom:state"

// Emitter pushes frontend subscription payloads into an attached desktop transport.
type Emitter interface {
	Emit(eventName string, payload any)
}

type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	InputDevice string `json:"inputDevice"`
	Language    string `json:"language"`
	Enabled     bool   `json:"enabled"`
}

type Highlight struct {
	Phrase string `json:"phrase"`
	Color  string `json:"color"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

type Entry struct {
	ID          string      `json:"id"`
	ChannelID   string      `json:"channelId"`
	ChannelName string      `json:"channelName"`
	Color       string      `json:"color"`
	Icon        string      `json:"icon"`
	Timestamp   string      `json:"timestamp"`
	Text        string      `json:"text"`
	Keywords    []string    `json:"keywords"`
	Highlights  []Highlight `json:"highlights"`
	Finalized   bool        `json:"finalized"`
}

type Partial struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	Timestamp   string `json:"timestamp"`
	Text        string `json:"text"`
}

type Snapshot struct {
	Entries  []Entry            `json:"entries"`
	Partials map[string]Partial `json:"partials"`
}

type BootstrapPayload struct {
	Channels     []Channel         `json:"channels"`
	InputDevices []Device          `json:"inputDevices"`
	AudioLevels  Levels            `json:"audioLevels"`
	Snapshot     Snapshot          `json:"snapshot"`
	Speech       SpeechDiagnostics `json:"speech"`
	Status       string            `json:"status"`
	EngineLabel  string            `json:"engineLabel"`
	KeywordCount int               `json:"keywordCount"`
}

type SubscriptionPayload struct {
	Channels     []Channel          `json:"channels,omitempty"`
	InputDevices []Device           `json:"inputDevices,omitempty"`
	AudioLevels  Levels             `json:"audioLevels,omitempty"`
	Snapshot     *Snapshot          `json:"snapshot,omitempty"`
	Speech       *SpeechDiagnostics `json:"speech,omitempty"`
	Status       string             `json:"status,omitempty"`
}

type Levels map[string]float64

type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ChannelUpdateInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	InputDevice string `json:"inputDevice"`
	Language    string `json:"language"`
	Enabled     bool   `json:"enabled"`
}

type SpeechDiagnostics struct {
	Model            string            `json:"model"`
	Task             string            `json:"task"`
	Temperature      float64           `json:"temperature"`
	BestOf           int               `json:"bestOf"`
	BeamSize         int               `json:"beamSize"`
	ChannelLanguages map[string]string `json:"channelLanguages"`
	LastChannelID    string            `json:"lastChannelId"`
	LastLanguage     string            `json:"lastLanguage"`
	LastInferenceMS  int               `json:"lastInferenceMs"`
	LastTextChars    int               `json:"lastTextChars"`
	LastError        string            `json:"lastError"`
	UpdatedAt        string            `json:"updatedAt"`
}

// Bridge exposes Wails-bindable methods and live update emission for the frontend.
type Bridge struct {
	config     config.Config
	bus        *events.Bus
	logger     *log.Logger
	channels   *channels.Manager
	audio      *audio.Manager
	speech     *speech.Manager
	transcript *transcript.Manager

	mu           sync.RWMutex
	emitter      Emitter
	subscription *events.Subscription
	started      bool
	cancel       context.CancelFunc
	workerDone   chan struct{}
}

func New(cfg config.Config, bus *events.Bus, logger *log.Logger, channelManager *channels.Manager, audioManager *audio.Manager, speechManager *speech.Manager, transcriptManager *transcript.Manager) (*Bridge, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if channelManager == nil {
		return nil, fmt.Errorf("channel manager is required")
	}
	if audioManager == nil {
		return nil, fmt.Errorf("audio manager is required")
	}
	if speechManager == nil {
		return nil, fmt.Errorf("speech manager is required")
	}
	if transcriptManager == nil {
		return nil, fmt.Errorf("transcript manager is required")
	}

	return &Bridge{
		config:     cfg,
		bus:        bus,
		logger:     logger,
		channels:   channelManager,
		audio:      audioManager,
		speech:     speechManager,
		transcript: transcriptManager,
	}, nil
}

func (b *Bridge) Name() string {
	return "frontend-bridge"
}

func (b *Bridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()
		return fmt.Errorf("frontend bridge already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	subscription, err := b.bus.Subscribe()
	if err != nil {
		cancel()
		b.mu.Unlock()
		return fmt.Errorf("subscribe to event bus: %w", err)
	}
	b.subscription = subscription
	b.cancel = cancel
	b.workerDone = make(chan struct{})
	b.started = true
	b.mu.Unlock()

	go b.consume(runCtx, subscription)
	return nil
}

func (b *Bridge) Stop(ctx context.Context) error {
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return nil
	}
	b.started = false
	cancel := b.cancel
	subscription := b.subscription
	workerDone := b.workerDone
	b.cancel = nil
	b.subscription = nil
	b.workerDone = nil
	b.mu.Unlock()

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

func (b *Bridge) AttachEmitter(emitter Emitter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitter = emitter
}

func (b *Bridge) GetBootstrap() (BootstrapPayload, error) {
	channelsPayload, inputDevices, audioLevels, snapshot, err := b.snapshotForFrontend()
	if err != nil {
		return BootstrapPayload{}, err
	}
	return BootstrapPayload{
		Channels:     channelsPayload,
		InputDevices: inputDevices,
		AudioLevels:  audioLevels,
		Snapshot:     snapshot,
		Speech:       b.speechDiagnostics(),
		Status:       "live",
		EngineLabel:  "Offline worker bridge",
		KeywordCount: len(b.config.Keywords),
	}, nil
}

func (b *Bridge) UpdateChannel(input ChannelUpdateInput) (Channel, error) {
	updated, err := b.channels.Update(channels.UpdateRequest{
		ID:          input.ID,
		Name:        input.Name,
		Color:       input.Color,
		Icon:        input.Icon,
		InputDevice: input.InputDevice,
		Language:    input.Language,
		Enabled:     input.Enabled,
	})
	if err != nil {
		return Channel{}, err
	}
	return mapChannel(updated), nil
}

func (b *Bridge) consume(ctx context.Context, subscription *events.Subscription) {
	defer close(b.workerDone)
	var lastAudioLevelsEmit time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			if !shouldEmit(event.Type) {
				continue
			}
			if event.Type == audio.EventTypeChunkCaptured {
				if time.Since(lastAudioLevelsEmit) < 125*time.Millisecond {
					continue
				}
				payload, err := b.frontendLevelsUpdate()
				if err != nil {
					b.logger.Printf("frontend bridge level snapshot failed: %v", err)
					continue
				}
				lastAudioLevelsEmit = time.Now()
				b.emit(payload)
				continue
			}

			if event.Type == speech.EventTypeEngineStateChanged {
				payload := SubscriptionPayload{Speech: pointerToDiagnostics(b.speechDiagnostics()), Status: "live"}
				b.emit(payload)
				continue
			}

			payload, err := b.frontendUpdate()
			if err != nil {
				b.logger.Printf("frontend bridge snapshot failed: %v", err)
				continue
			}
			b.emit(payload)
		}
	}
}

func shouldEmit(eventType string) bool {
	switch eventType {
	case channels.EventTypeUpdated, transcript.EventTypeUpdated, audio.EventTypeDevicesUpdated, audio.EventTypeChunkCaptured, speech.EventTypeEngineStateChanged:
		return true
	default:
		return false
	}
}

func (b *Bridge) frontendUpdate() (SubscriptionPayload, error) {
	channelsPayload, inputDevices, audioLevels, snapshot, err := b.snapshotForFrontend()
	if err != nil {
		return SubscriptionPayload{}, err
	}
	return SubscriptionPayload{Channels: channelsPayload, InputDevices: inputDevices, AudioLevels: audioLevels, Snapshot: &snapshot, Speech: pointerToDiagnostics(b.speechDiagnostics()), Status: "live"}, nil
}

func (b *Bridge) frontendLevelsUpdate() (SubscriptionPayload, error) {
	audioLevels := b.audio.Levels()
	return SubscriptionPayload{AudioLevels: audioLevels, Status: "live"}, nil
}

func (b *Bridge) speechDiagnostics() SpeechDiagnostics {
	diagnostics := b.speech.Diagnostics()
	channelLanguages := make(map[string]string, len(diagnostics.ChannelLanguages))
	for key, value := range diagnostics.ChannelLanguages {
		channelLanguages[key] = value
	}

	return SpeechDiagnostics{
		Model:            diagnostics.Model,
		Task:             diagnostics.Task,
		Temperature:      diagnostics.Temperature,
		BestOf:           diagnostics.BestOf,
		BeamSize:         diagnostics.BeamSize,
		ChannelLanguages: channelLanguages,
		LastChannelID:    diagnostics.LastChannelID,
		LastLanguage:     diagnostics.LastLanguage,
		LastInferenceMS:  diagnostics.LastInferenceMS,
		LastTextChars:    diagnostics.LastTextChars,
		LastError:        diagnostics.LastError,
		UpdatedAt:        formatTime(diagnostics.UpdatedAt),
	}
}

func pointerToDiagnostics(value SpeechDiagnostics) *SpeechDiagnostics {
	return &value
}

func (b *Bridge) emit(payload SubscriptionPayload) {
	b.mu.RLock()
	emitter := b.emitter
	b.mu.RUnlock()
	if emitter == nil {
		return
	}
	emitter.Emit(EventName, payload)
}

func (b *Bridge) snapshotForFrontend() ([]Channel, []Device, Levels, Snapshot, error) {
	channelSnapshot := b.channels.Snapshot()
	deviceInventory := b.audio.Inventory()
	audioLevels := b.audio.Levels()
	transcriptSnapshot := b.transcript.Snapshot()

	channelsPayload := make([]Channel, 0, len(channelSnapshot.Channels))
	channelByID := make(map[string]Channel, len(channelSnapshot.Channels))
	for _, channel := range channelSnapshot.Channels {
		mapped := mapChannel(channel)
		channelsPayload = append(channelsPayload, mapped)
		channelByID[channel.ID] = mapped
	}

	entries := make([]Entry, 0, len(transcriptSnapshot.Entries))
	for _, entry := range transcriptSnapshot.Entries {
		metadata := channelByID[entry.ChannelID]
		entries = append(entries, mapEntry(entry, metadata))
	}

	partials := make(map[string]Partial, len(transcriptSnapshot.Partials))
	for channelID, partial := range transcriptSnapshot.Partials {
		metadata := channelByID[channelID]
		partials[channelID] = Partial{
			ChannelID:   partial.ChannelID,
			ChannelName: fallback(metadata.Name, partial.ChannelName),
			Color:       metadata.Color,
			Icon:        metadata.Icon,
			Timestamp:   formatTime(partial.Timestamp),
			Text:        partial.Text,
		}
	}

	inputDevices := make([]Device, 0, len(deviceInventory.Devices))
	for _, device := range deviceInventory.Devices {
		inputDevices = append(inputDevices, Device{ID: device.ID, Name: device.Name})
	}

	return channelsPayload, inputDevices, audioLevels, Snapshot{Entries: entries, Partials: partials}, nil
}

func mapChannel(channel channels.Channel) Channel {
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

func mapEntry(entry transcript.Entry, metadata Channel) Entry {
	highlights := make([]Highlight, 0, len(entry.Highlights))
	for _, highlight := range entry.Highlights {
		highlights = append(highlights, Highlight{
			Phrase: highlight.Phrase,
			Color:  highlight.Color,
			Start:  highlight.Start,
			End:    highlight.End,
		})
	}

	keywords := append([]string{}, entry.Keywords...)
	if len(highlights) == 0 {
		highlights = []Highlight{}
	}

	return Entry{
		ID:          entry.ID,
		ChannelID:   entry.ChannelID,
		ChannelName: fallback(metadata.Name, entry.ChannelName),
		Color:       metadata.Color,
		Icon:        metadata.Icon,
		Timestamp:   formatTime(entry.Timestamp),
		Text:        entry.Text,
		Keywords:    keywords,
		Highlights:  highlights,
		Finalized:   true,
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("15:04:05")
}

func fallback(primary string, secondary string) string {
	if primary != "" {
		return primary
	}
	return secondary
}

func ComponentFactory(bridge *Bridge) app.ComponentFactory {
	return func(app.Dependencies) (app.Component, error) {
		return bridge, nil
	}
}
