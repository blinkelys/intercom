package audio

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
)

const (
	defaultSampleRate      = 16000
	defaultFramesPerBuffer = 1024
	defaultRingBufferSize  = defaultSampleRate * 5
	defaultRecoveryTick    = 3 * time.Second
)

// Manager owns audio device discovery and one capture session per configured channel.
type Manager struct {
	config             config.Config
	bus                *events.Bus
	logger             *log.Logger
	driver             Driver
	sampleRate         int
	framesPerBuffer    int
	ringBufferCapacity int
	recoveryTick       time.Duration

	mu        sync.RWMutex
	sessions  map[string]*session
	channels  map[string]config.ChannelConfig
	inventory DeviceInventory
	sub       *events.Subscription
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	started   bool
}

type session struct {
	channel config.ChannelConfig
	stream  Stream
	buffer  *RingBuffer
}

// ManagerOption configures audio manager behavior.
type ManagerOption func(*Manager)

// NewManager constructs an audio manager using the provided runtime dependencies.
func NewManager(cfg config.Config, bus *events.Bus, logger *log.Logger, driver Driver, opts ...ManagerOption) (*Manager, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if driver == nil {
		driver = NullDriver{}
	}

	manager := &Manager{
		config:             cfg,
		bus:                bus,
		logger:             logger,
		driver:             driver,
		sampleRate:         defaultSampleRate,
		framesPerBuffer:    defaultFramesPerBuffer,
		ringBufferCapacity: defaultRingBufferSize,
		recoveryTick:       defaultRecoveryTick,
		sessions:           make(map[string]*session),
		channels:           make(map[string]config.ChannelConfig, len(cfg.Channels)),
	}

	for _, channel := range cfg.Channels {
		manager.channels[channel.ID] = channel
	}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	if manager.framesPerBuffer <= 0 {
		return nil, fmt.Errorf("frames per buffer must be positive")
	}
	if manager.ringBufferCapacity <= 0 {
		return nil, fmt.Errorf("ring buffer capacity must be positive")
	}
	if manager.recoveryTick <= 0 {
		return nil, fmt.Errorf("recovery tick must be positive")
	}

	return manager, nil
}

// Name returns the lifecycle component name.
func (m *Manager) Name() string {
	return "audio-manager"
}

// Start enumerates devices and starts one independent pipeline per valid channel.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("audio manager already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.started = true
	m.mu.Unlock()

	if err := m.RefreshInventory(runCtx); err != nil {
		m.logger.Printf("audio device enumeration failed: %v", err)
	}
	devices := m.Inventory().Devices

	deviceByID := make(map[string]Device, len(devices))
	availableIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		deviceByID[device.ID] = device
		availableIDs = append(availableIDs, device.ID)
	}
	usedDeviceIDs := make(map[string]struct{})

	channelSnapshot := m.channelSnapshot()
	for _, channel := range channelSnapshot {
		allowAutoAssign := strings.TrimSpace(channel.InputDevice) != ""
		resolved, statusState, statusMessage := resolveChannelDevice(channel, availableIDs, usedDeviceIDs, deviceByID, allowAutoAssign)
		if statusState != "" {
			m.publishStatus(channel.ID, resolved.InputDevice, statusState, statusMessage)
			continue
		}
		usedDeviceIDs[resolved.InputDevice] = struct{}{}

		if err := m.startSession(runCtx, resolved); err != nil {
			m.publishStatus(channel.ID, resolved.InputDevice, PipelineStateFailed, err.Error())
			m.logger.Printf("audio pipeline start failed channel=%s: %v", channel.ID, err)
		}
	}

	subscription, err := m.bus.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe to channel updates: %w", err)
	}
	m.mu.Lock()
	m.sub = subscription
	m.mu.Unlock()

	m.wg.Add(1)
	go m.consumeEvents(runCtx, subscription)

	return nil
}

// RefreshInventory re-enumerates available input devices and updates inventory state.
func (m *Manager) RefreshInventory(ctx context.Context) error {
	devices, err := m.driver.Devices(ctx)
	if err != nil {
		m.mu.RLock()
		existing := append([]Device(nil), m.inventory.Devices...)
		m.mu.RUnlock()
		m.publishInventory(existing, err)
		return err
	}
	m.publishInventory(devices, nil)
	return nil
}

// Stop shuts down all active channel pipelines.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	cancel := m.cancel
	subscription := m.sub
	sessions := make(map[string]*session, len(m.sessions))
	for channelID, current := range m.sessions {
		sessions[channelID] = current
	}
	m.started = false
	m.cancel = nil
	m.sub = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if subscription != nil {
		subscription.Close()
	}

	var stopErr error
	for channelID, current := range sessions {
		if err := current.stream.Stop(ctx); err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("stop stream for channel %q: %w", channelID, err))
		}
	}

	stopped := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		stopErr = errors.Join(stopErr, ctx.Err())
	case <-stopped:
	}

	m.mu.Lock()
	m.sessions = make(map[string]*session)
	m.mu.Unlock()

	return stopErr
}

// Levels returns normalized per-channel audio levels from recent buffered frames.
func (m *Manager) Levels() map[string]float64 {
	m.mu.RLock()
	sessions := make(map[string]*session, len(m.sessions))
	for channelID, current := range m.sessions {
		sessions[channelID] = current
	}
	m.mu.RUnlock()

	levels := make(map[string]float64, len(sessions))
	for channelID, current := range sessions {
		frames := current.buffer.Snapshot(m.framesPerBuffer)
		levels[channelID] = levelFromFrames(frames)
	}
	return levels
}

// Snapshot returns up to the newest maxFrames retained for the given channel.
func (m *Manager) Snapshot(channelID string, maxFrames int) ([]float32, bool) {
	m.mu.RLock()
	current, ok := m.sessions[channelID]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}

	return current.buffer.Snapshot(maxFrames), true
}

// Inventory returns the latest discovered audio input devices.
func (m *Manager) Inventory() DeviceInventory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copyDevices := append([]Device(nil), m.inventory.Devices...)
	return DeviceInventory{
		Devices:     copyDevices,
		RefreshedAt: m.inventory.RefreshedAt,
		Error:       m.inventory.Error,
	}
}

// WithSampleRate overrides the target capture sample rate.
func WithSampleRate(sampleRate int) ManagerOption {
	return func(manager *Manager) {
		manager.sampleRate = sampleRate
	}
}

// WithFramesPerBuffer overrides the target capture buffer size.
func WithFramesPerBuffer(framesPerBuffer int) ManagerOption {
	return func(manager *Manager) {
		manager.framesPerBuffer = framesPerBuffer
	}
}

// WithRingBufferCapacity overrides the number of retained audio frames per channel.
func WithRingBufferCapacity(capacity int) ManagerOption {
	return func(manager *Manager) {
		manager.ringBufferCapacity = capacity
	}
}

// WithRecoveryTick overrides the periodic pipeline recovery interval.
func WithRecoveryTick(interval time.Duration) ManagerOption {
	return func(manager *Manager) {
		manager.recoveryTick = interval
	}
}

func (m *Manager) startSession(ctx context.Context, channel config.ChannelConfig) error {
	m.publishStatus(channel.ID, channel.InputDevice, PipelineStateStarting, "starting capture pipeline")

	stream, err := m.driver.OpenStream(ctx, StreamConfig{
		ChannelID:       channel.ID,
		DeviceID:        channel.InputDevice,
		SampleRate:      m.sampleRate,
		FramesPerBuffer: m.framesPerBuffer,
	})
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	buffer, err := NewRingBuffer(m.ringBufferCapacity)
	if err != nil {
		return fmt.Errorf("allocate ring buffer: %w", err)
	}

	if err := stream.Start(ctx); err != nil {
		_ = stream.Stop(ctx)
		return fmt.Errorf("start stream: %w", err)
	}

	current := &session{
		channel: channel,
		stream:  stream,
		buffer:  buffer,
	}

	m.mu.Lock()
	m.sessions[channel.ID] = current
	m.mu.Unlock()

	m.wg.Add(1)
	go m.consumeSession(ctx, current)

	m.publishStatus(channel.ID, channel.InputDevice, PipelineStateRunning, "capture pipeline running")
	return nil
}

func (m *Manager) consumeSession(ctx context.Context, current *session) {
	defer m.wg.Done()
	defer m.publishStatus(current.channel.ID, current.channel.InputDevice, PipelineStateStopped, "capture pipeline stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-current.stream.Chunks():
			if !ok {
				return
			}

			if chunk.ChannelID == "" {
				chunk.ChannelID = current.channel.ID
			}
			if chunk.SampleRate == 0 {
				chunk.SampleRate = m.sampleRate
			}
			if chunk.CapturedAt.IsZero() {
				chunk.CapturedAt = time.Now().UTC()
			}

			if current.channel.GainDB != 0 {
				chunk.Frames = applyGainDB(chunk.Frames, current.channel.GainDB)
			}

			current.buffer.Write(chunk.Frames)
			m.bus.Publish(events.Event{Type: EventTypeChunkCaptured, Payload: chunk})
		}
	}
}

func applyGainDB(frames []float32, gainDB float64) []float32 {
	if len(frames) == 0 || gainDB == 0 {
		return frames
	}
	multiplier := math.Pow(10, gainDB/20)
	if multiplier == 1 {
		return frames
	}

	adjusted := make([]float32, len(frames))
	for index, frame := range frames {
		value := float64(frame) * multiplier
		if value > 1 {
			value = 1
		} else if value < -1 {
			value = -1
		}
		adjusted[index] = float32(value)
	}
	return adjusted
}

func (m *Manager) consumeEvents(ctx context.Context, subscription *events.Subscription) {
	defer m.wg.Done()
	recoveryTicker := time.NewTicker(m.recoveryTick)
	defer recoveryTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-recoveryTicker.C:
			if err := m.RefreshInventory(ctx); err != nil {
				m.logger.Printf("audio periodic inventory refresh failed: %v", err)
			}
			m.reconcileAll(ctx)
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			switch event.Type {
			case channels.EventTypeUpdated:
				update, ok := event.Payload.(channels.Update)
				if !ok {
					continue
				}
				channel := toChannelConfig(update.Channel)
				m.mu.Lock()
				m.channels[channel.ID] = channel
				m.mu.Unlock()
				m.applyChannelUpdate(ctx, channel)
			case channels.EventTypeDeleted:
				deleted, ok := event.Payload.(channels.Deleted)
				if !ok {
					continue
				}
				m.mu.Lock()
				delete(m.channels, deleted.ChannelID)
				m.mu.Unlock()
				if err := m.stopSession(ctx, deleted.ChannelID); err != nil {
					m.publishStatus(deleted.ChannelID, "", PipelineStateFailed, err.Error())
				}
			case EventTypeDevicesUpdated:
				m.reconcileAll(ctx)
			default:
				continue
			}
		}
	}
}

func (m *Manager) reconcileAll(ctx context.Context) {
	for _, channel := range m.channelSnapshot() {
		m.applyChannelUpdate(ctx, channel)
	}
}

func (m *Manager) channelSnapshot() []config.ChannelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make([]config.ChannelConfig, 0, len(m.channels))
	for _, channel := range m.channels {
		snapshot = append(snapshot, channel)
	}
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].ID < snapshot[j].ID
	})
	return snapshot
}

func (m *Manager) applyChannelUpdate(ctx context.Context, channel config.ChannelConfig) {
	m.mu.RLock()
	deviceByID := make(map[string]Device, len(m.inventory.Devices))
	availableIDs := make([]string, 0, len(m.inventory.Devices))
	usedDeviceIDs := make(map[string]struct{}, len(m.sessions))
	for _, device := range m.inventory.Devices {
		deviceByID[device.ID] = device
		availableIDs = append(availableIDs, device.ID)
	}
	for channelID, current := range m.sessions {
		if channelID == channel.ID {
			continue
		}
		if current.channel.InputDevice != "" {
			usedDeviceIDs[current.channel.InputDevice] = struct{}{}
		}
	}
	currentSession, hasSession := m.sessions[channel.ID]
	m.mu.RUnlock()

	// Protect active pipelines from accidental empty-device updates.
	if hasSession && channel.Enabled && strings.TrimSpace(channel.InputDevice) == "" {
		channel.InputDevice = currentSession.channel.InputDevice
	}

	allowAutoAssign := !hasSession && strings.TrimSpace(channel.InputDevice) != ""
	resolved, statusState, statusMessage := resolveChannelDevice(channel, availableIDs, usedDeviceIDs, deviceByID, allowAutoAssign)
	if statusState != "" {
		if hasSession {
			_ = m.stopSession(ctx, channel.ID)
		}
		m.publishStatus(channel.ID, resolved.InputDevice, statusState, statusMessage)
		return
	}

	if hasSession && currentSession.channel.InputDevice == resolved.InputDevice {
		m.mu.Lock()
		if existing, ok := m.sessions[channel.ID]; ok {
			existing.channel = resolved
		}
		m.mu.Unlock()
		return
	}

	if hasSession {
		if err := m.stopSession(ctx, channel.ID); err != nil {
			m.publishStatus(channel.ID, resolved.InputDevice, PipelineStateFailed, err.Error())
			return
		}
	}

	if err := m.startSession(ctx, resolved); err != nil {
		m.publishStatus(channel.ID, resolved.InputDevice, PipelineStateFailed, err.Error())
		m.logger.Printf("audio pipeline start failed channel=%s: %v", channel.ID, err)
	}
}

func (m *Manager) stopSession(ctx context.Context, channelID string) error {
	m.mu.Lock()
	current, ok := m.sessions[channelID]
	if ok {
		delete(m.sessions, channelID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if err := current.stream.Stop(ctx); err != nil {
		return fmt.Errorf("stop stream: %w", err)
	}
	return nil
}

func resolveChannelDevice(channel config.ChannelConfig, availableIDs []string, usedDeviceIDs map[string]struct{}, deviceByID map[string]Device, allowAutoAssign bool) (config.ChannelConfig, PipelineState, string) {
	resolved := channel
	if !resolved.Enabled {
		return resolved, PipelineStateDisabled, "channel disabled"
	}

	deviceID := strings.TrimSpace(resolved.InputDevice)
	if deviceID == "" {
		if !allowAutoAssign {
			return resolved, PipelineStateUnassigned, "no input device assigned"
		}
		deviceID = firstUnusedDevice(availableIDs, usedDeviceIDs)
		if deviceID == "" {
			return resolved, PipelineStateUnassigned, "no input device assigned"
		}
		resolved.InputDevice = deviceID
	}

	if _, ok := deviceByID[deviceID]; !ok {
		if allowAutoAssign {
			reassigned := firstUnusedDevice(availableIDs, usedDeviceIDs)
			if reassigned != "" {
				resolved.InputDevice = reassigned
				return resolved, "", ""
			}
		}
		return resolved, PipelineStateMissingDevice, "configured input device is unavailable"
	}

	return resolved, "", ""
}

func toChannelConfig(channel channels.Channel) config.ChannelConfig {
	return config.ChannelConfig{
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

func levelFromFrames(frames []float32) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sumSquares float64
	for _, frame := range frames {
		sample := float64(frame)
		sumSquares += sample * sample
	}
	rms := math.Sqrt(sumSquares / float64(len(frames)))
	level := math.Sqrt(rms * 120)
	if rms > 0 && level < 0.01 {
		level = 0.01
	}
	if level > 1 {
		return 1
	}
	return level
}

func (m *Manager) publishInventory(devices []Device, err error) {
	payload := DeviceInventory{
		Devices:     append([]Device(nil), devices...),
		RefreshedAt: time.Now().UTC(),
	}
	if err != nil {
		payload.Error = err.Error()
	}

	m.mu.Lock()
	m.inventory = DeviceInventory{
		Devices:     append([]Device(nil), payload.Devices...),
		RefreshedAt: payload.RefreshedAt,
		Error:       payload.Error,
	}
	m.mu.Unlock()

	m.bus.Publish(events.Event{Type: EventTypeDevicesUpdated, Payload: payload})
}

func (m *Manager) publishStatus(channelID string, deviceID string, state PipelineState, message string) {
	m.bus.Publish(events.Event{Type: EventTypePipelineStateChanged, Payload: PipelineStatus{
		ChannelID:  channelID,
		DeviceID:   deviceID,
		State:      state,
		OccurredAt: time.Now().UTC(),
		Message:    message,
	}})
}

func firstUnusedDevice(deviceIDs []string, used map[string]struct{}) string {
	for _, deviceID := range deviceIDs {
		if _, exists := used[deviceID]; !exists {
			return deviceID
		}
	}
	return ""
}
