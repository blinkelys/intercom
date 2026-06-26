package audio

import (
	"context"
	"io"
	"log"
	"reflect"
	"testing"
	"time"

	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
)

func TestManagerAutoAssignsDeviceAndReportsMissingDevices(t *testing.T) {
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
		devices: []Device{{ID: "device-1", Name: "Mic 1"}},
		streams: map[string]*fakeStream{"producer": newFakeStream()},
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
	if !containsState(states, "producer", PipelineStateRunning) {
		t.Fatalf("expected running state in %v", states)
	}
	if !containsState(states, "md", PipelineStateMissingDevice) {
		t.Fatalf("expected missing-device state in %v", states)
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

func TestManagerUnassignUpdateStopsChannelPipeline(t *testing.T) {
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

	waitForPipelineState(t, subscription.Events(), "producer", PipelineStateUnassigned)

	if levels := manager.Levels(); len(levels) != 0 {
		t.Fatalf("levels = %#v, want empty after unassign", levels)
	}
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
