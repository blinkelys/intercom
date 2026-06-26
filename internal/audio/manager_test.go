package audio

import (
	"context"
	"errors"
	"io"
	"log"
	"reflect"
	"testing"
	"time"

	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
)

func TestManagerKeepsUnassignedAndRecoversStaleConfiguredDevices(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{
			{ID: "producer", Name: "Producer", Enabled: true},
			{ID: "md", Name: "Musical Director", Enabled: true, InputDevice: "missing-device"},
		},
	}, bus, log.New(io.Discard, "", 0), fakeDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}, {ID: "device-2", Name: "Mic 2"}},
		streams: map[string]*fakeStream{"producer": newFakeStream(), "md": newFakeStream()},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	states := collectPipelineStates(t, subscription.Events(), 3)
	if !containsState(states, "producer", PipelineStateUnassigned) {
		t.Fatalf("expected unassigned state in %v", states)
	}
	if !containsState(states, "md", PipelineStateRunning) {
		t.Fatalf("expected running state in %v", states)
	}
}

func TestManagerConsumesChunksAndRetainsSnapshot(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	stream := newFakeStream(SampleChunk{Frames: []float32{0.1, 0.2, 0.3}})
	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "device-1",
		}},
	}, bus, log.New(io.Discard, "", 0), fakeDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}},
		streams: map[string]*fakeStream{"producer": stream},
	}, WithRingBufferCapacity(8))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	chunkEvent := waitForEventType(t, subscription.Events(), EventTypeChunkCaptured)
	chunk, ok := chunkEvent.Payload.(SampleChunk)
	if !ok {
		t.Fatalf("payload type = %T, want SampleChunk", chunkEvent.Payload)
	}

	if chunk.ChannelID != "producer" {
		t.Fatalf("chunk channel = %q, want producer", chunk.ChannelID)
	}

	frames, found := manager.Snapshot("producer", 0)
	if !found {
		t.Fatal("expected snapshot for producer")
	}
	want := []float32{0.1, 0.2, 0.3}
	if !reflect.DeepEqual(frames, want) {
		t.Fatalf("snapshot = %v, want %v", frames, want)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("stop manager: %v", err)
	}
}

func TestManagerReconfiguresPipelineOnChannelUpdate(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{
			{ID: "producer", Name: "Producer", Enabled: true, InputDevice: "device-1"},
			{ID: "md", Name: "Musical Director", Enabled: false},
		},
	}, bus, log.New(io.Discard, "", 0), fakeDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}, {ID: "device-2", Name: "Mic 2"}},
		streams: map[string]*fakeStream{
			"producer": newFakeStream(),
			"md":       newFakeStream(SampleChunk{Frames: []float32{0.4, 0.5}}),
		},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	bus.Publish(events.Event{Type: channels.EventTypeUpdated, Payload: channels.Update{
		Channel: channels.Channel{
			ID:          "md",
			Name:        "Musical Director",
			Color:       "#22C55E",
			Icon:        "🎼",
			InputDevice: "device-2",
			Language:    "en",
			Enabled:     true,
		},
		OccurredAt: time.Now().UTC(),
	}})

	waitForPipelineState(t, subscription.Events(), "md", PipelineStateRunning)
	chunk := waitForChunk(t, subscription.Events(), "md")
	if len(chunk.Frames) == 0 {
		t.Fatal("expected captured frames for md channel")
	}
}

func TestManagerEmptyInputUpdateKeepsExistingPipeline(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "device-1",
		}},
	}, bus, log.New(io.Discard, "", 0), fakeDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}},
		streams: map[string]*fakeStream{"producer": newFakeStream(SampleChunk{Frames: []float32{0.2}})},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateRunning)

	bus.Publish(events.Event{Type: channels.EventTypeUpdated, Payload: channels.Update{
		Channel: channels.Channel{
			ID:          "producer",
			Name:        "Producer",
			Color:       "#EF4444",
			Icon:        "🎬",
			InputDevice: "",
			Language:    "en",
			Enabled:     true,
		},
		OccurredAt: time.Now().UTC(),
	}})

	time.Sleep(100 * time.Millisecond)
	if levels := manager.Levels(); len(levels) == 0 {
		t.Fatalf("levels = %#v, want channel to stay active", levels)
	}
}

func TestManagerRefreshInventoryPreservesExistingDevicesOnFailure(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}

	driver := &sequenceDriver{
		calls: []deviceCall{
			{devices: []Device{{ID: "device-1", Name: "Mic 1"}}},
			{err: errors.New("helper timeout")},
		},
		streams: map[string]*fakeStream{
			"producer": newFakeStream(SampleChunk{Frames: []float32{0.2}}),
		},
	}

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "device-1",
		}},
	}, bus, log.New(io.Discard, "", 0), driver)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	if err := manager.RefreshInventory(ctx); err == nil {
		t.Fatal("expected refresh inventory error")
	}

	inventory := manager.Inventory()
	if len(inventory.Devices) != 1 || inventory.Devices[0].ID != "device-1" {
		t.Fatalf("inventory devices = %#v, want preserved existing devices", inventory.Devices)
	}
}

func TestManagerRecoversPipelinesAfterStartupInventoryFailure(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	driver := &sequenceDriver{
		calls: []deviceCall{
			{err: errors.New("enumeration failed")},
			{devices: []Device{{ID: "device-1", Name: "Mic 1"}}},
		},
		streams: map[string]*fakeStream{
			"producer": newFakeStream(SampleChunk{Frames: []float32{0.3}}),
		},
	}

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "device-1",
		}},
	}, bus, log.New(io.Discard, "", 0), driver)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateMissingDevice)

	if err := manager.RefreshInventory(ctx); err != nil {
		t.Fatalf("refresh inventory: %v", err)
	}

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateRunning)
	_ = waitForChunk(t, subscription.Events(), "producer")
}

func TestManagerRecoversStaleConfiguredChannelAfterInventoryBecomesAvailable(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	driver := &sequenceDriver{
		calls: []deviceCall{
			{err: errors.New("enumeration failed")},
			{devices: []Device{{ID: "device-1", Name: "Mic 1"}}},
		},
		streams: map[string]*fakeStream{
			"producer": newFakeStream(SampleChunk{Frames: []float32{0.9}}),
		},
	}

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "stale-device-id",
		}},
	}, bus, log.New(io.Discard, "", 0), driver)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateMissingDevice)

	if err := manager.RefreshInventory(ctx); err != nil {
		t.Fatalf("refresh inventory: %v", err)
	}

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateRunning)
	_ = waitForChunk(t, subscription.Events(), "producer")
}

func TestManagerAutoAssignsWhenConfiguredDeviceIsUnavailable(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "stale-device-id",
		}},
	}, bus, log.New(io.Discard, "", 0), fakeDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}},
		streams: map[string]*fakeStream{"producer": newFakeStream(SampleChunk{Frames: []float32{0.6}})},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateRunning)
	_ = waitForChunk(t, subscription.Events(), "producer")
}

func TestManagerPeriodicRecoveryRestartsFailedPipeline(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(64)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	driver := &transientOpenDriver{
		devices: []Device{{ID: "device-1", Name: "Mic 1"}},
		failOpen: map[string]int{
			"producer": 1,
		},
		streams: map[string]*fakeStream{
			"producer": newFakeStream(SampleChunk{Frames: []float32{0.7}}),
		},
	}

	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{
			ID:          "producer",
			Name:        "Producer",
			Enabled:     true,
			InputDevice: "device-1",
		}},
	}, bus, log.New(io.Discard, "", 0), driver, WithRecoveryTick(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatalf("stop manager: %v", err)
		}
	}()

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateFailed)
	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateRunning)
}

type fakeDriver struct {
	devices []Device
	streams map[string]*fakeStream
}

func (d fakeDriver) Devices(context.Context) ([]Device, error) {
	return append([]Device(nil), d.devices...), nil
}

func (d fakeDriver) OpenStream(_ context.Context, cfg StreamConfig) (Stream, error) {
	if stream, ok := d.streams[cfg.ChannelID]; ok {
		return stream, nil
	}
	return nil, errStreamUnsupported
}

type deviceCall struct {
	devices []Device
	err     error
}

type sequenceDriver struct {
	calls   []deviceCall
	index   int
	streams map[string]*fakeStream
}

func (d *sequenceDriver) Devices(context.Context) ([]Device, error) {
	if len(d.calls) == 0 {
		return nil, nil
	}
	if d.index >= len(d.calls) {
		last := d.calls[len(d.calls)-1]
		return append([]Device(nil), last.devices...), last.err
	}
	call := d.calls[d.index]
	d.index += 1
	return append([]Device(nil), call.devices...), call.err
}

func (d *sequenceDriver) OpenStream(_ context.Context, cfg StreamConfig) (Stream, error) {
	if stream, ok := d.streams[cfg.ChannelID]; ok {
		return stream, nil
	}
	return nil, errStreamUnsupported
}

type transientOpenDriver struct {
	devices  []Device
	failOpen map[string]int
	streams  map[string]*fakeStream
}

func (d *transientOpenDriver) Devices(context.Context) ([]Device, error) {
	return append([]Device(nil), d.devices...), nil
}

func (d *transientOpenDriver) OpenStream(_ context.Context, cfg StreamConfig) (Stream, error) {
	if remaining := d.failOpen[cfg.ChannelID]; remaining > 0 {
		d.failOpen[cfg.ChannelID] = remaining - 1
		return nil, errors.New("transient open failure")
	}
	if stream, ok := d.streams[cfg.ChannelID]; ok {
		return stream, nil
	}
	return nil, errStreamUnsupported
}

type fakeStream struct {
	chunks chan SampleChunk
	seed   []SampleChunk
	closed bool
}

func newFakeStream(seed ...SampleChunk) *fakeStream {
	return &fakeStream{
		chunks: make(chan SampleChunk, len(seed)),
		seed:   append([]SampleChunk(nil), seed...),
	}
}

func (s *fakeStream) Start(context.Context) error {
	for _, chunk := range s.seed {
		s.chunks <- chunk
	}
	close(s.chunks)
	return nil
}

func (s *fakeStream) Chunks() <-chan SampleChunk {
	return s.chunks
}

func (s *fakeStream) Stop(context.Context) error {
	if s.closed {
		return nil
	}
	s.closed = true
	return nil
}

func collectPipelineStates(t *testing.T, eventsCh <-chan events.Event, expected int) []PipelineStatus {
	t.Helper()

	states := make([]PipelineStatus, 0, expected)
	timeout := time.After(time.Second)
	for len(states) < expected {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for %d pipeline states", expected)
		case event := <-eventsCh:
			if event.Type != EventTypePipelineStateChanged {
				continue
			}
			status, ok := event.Payload.(PipelineStatus)
			if !ok {
				t.Fatalf("payload type = %T, want PipelineStatus", event.Payload)
			}
			states = append(states, status)
		}
	}

	return states
}

func containsState(states []PipelineStatus, channelID string, want PipelineState) bool {
	for _, state := range states {
		if state.ChannelID == channelID && state.State == want {
			return true
		}
	}
	return false
}

func waitForEventType(t *testing.T, eventsCh <-chan events.Event, eventType string) events.Event {
	t.Helper()

	timeout := time.After(time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for event %q", eventType)
		case event := <-eventsCh:
			if event.Type == eventType {
				return event
			}
		}
	}
}

func waitForPipelineState(t *testing.T, eventsCh <-chan events.Event, channelID string, want PipelineState) {
	t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for channel %q state %q", channelID, want)
		case event := <-eventsCh:
			if event.Type != EventTypePipelineStateChanged {
				continue
			}
			status, ok := event.Payload.(PipelineStatus)
			if !ok {
				continue
			}
			if status.ChannelID == channelID && status.State == want {
				return
			}
		}
	}
}

func waitForChunk(t *testing.T, eventsCh <-chan events.Event, channelID string) SampleChunk {
	t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for chunk for channel %q", channelID)
		case event := <-eventsCh:
			if event.Type != EventTypeChunkCaptured {
				continue
			}
			chunk, ok := event.Payload.(SampleChunk)
			if !ok {
				continue
			}
			if chunk.ChannelID == channelID {
				return chunk
			}
		}
	}
}
