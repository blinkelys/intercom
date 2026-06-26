package speech

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"procom/internal/audio"
	"procom/internal/config"
	"procom/internal/events"
)

func TestManagerPublishesFinalResultsFromEngine(t *testing.T) {
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

	engine := newFakeEngine()
	manager, err := NewManager(config.Config{Speech: config.SpeechConfig{Enabled: true, Engine: "mlx_whisper"}}, bus, log.New(io.Discard, "", 0), func(config.SpeechConfig, *log.Logger) (Engine, error) {
		return engine, nil
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

	bus.Publish(events.Event{Type: audio.EventTypeChunkCaptured, Payload: audio.SampleChunk{ChannelID: "producer", SampleRate: 16000, Frames: []float32{0.1}}})
	engine.results <- Result{ChannelID: "producer", Text: "go now", Final: true}
	waitForSubmittedChunk(t, engine, "producer")

	event := waitForSpeechEvent(t, subscription.Events(), EventTypeResultFinal)
	result, ok := event.Payload.(Result)
	if !ok {
		t.Fatalf("payload type = %T, want Result", event.Payload)
	}
	if result.Text != "go now" {
		t.Fatalf("result text = %q, want %q", result.Text, "go now")
	}

	if len(engine.submitted) == 0 || engine.submitted[0].ChannelID != "producer" {
		t.Fatalf("submitted chunks = %#v, want producer chunk", engine.submitted)
	}
}

func TestManagerPublishesDisabledStatusWhenSpeechDisabled(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(8)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	manager, err := NewManager(config.Config{Speech: config.SpeechConfig{Enabled: false, Engine: "mlx_whisper"}}, bus, log.New(io.Discard, "", 0), nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	event := waitForSpeechEvent(t, subscription.Events(), EventTypeEngineStateChanged)
	status, ok := event.Payload.(Status)
	if !ok {
		t.Fatalf("payload type = %T, want Status", event.Payload)
	}
	if status.State != EngineStateDisabled {
		t.Fatalf("status state = %q, want %q", status.State, EngineStateDisabled)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("stop manager: %v", err)
	}
}

func TestManagerBuffersShortFinalFragmentsUntilTheyFormSentence(t *testing.T) {
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

	engine := newFakeEngine()
	manager, err := NewManager(config.Config{Speech: config.SpeechConfig{Enabled: true, Engine: "mlx_whisper"}}, bus, log.New(io.Discard, "", 0), func(config.SpeechConfig, *log.Logger) (Engine, error) {
		return engine, nil
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

	engine.results <- Result{ChannelID: "producer", Text: "stand by", Final: true, ReceivedAt: time.Now().UTC()}
	engine.results <- Result{ChannelID: "producer", Text: "for deck two now and hold", Final: true, ReceivedAt: time.Now().UTC()}

	event := waitForSpeechEvent(t, subscription.Events(), EventTypeResultFinal)
	result, ok := event.Payload.(Result)
	if !ok {
		t.Fatalf("payload type = %T, want Result", event.Payload)
	}
	if result.ChannelID != "producer" {
		t.Fatalf("result channel = %q, want producer", result.ChannelID)
	}
	if result.Text != "stand by for deck two now and hold" {
		t.Fatalf("result text = %q, want merged sentence", result.Text)
	}
}

func TestManagerPublishesPunctuatedFinalImmediately(t *testing.T) {
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

	engine := newFakeEngine()
	manager, err := NewManager(config.Config{Speech: config.SpeechConfig{Enabled: true, Engine: "mlx_whisper"}}, bus, log.New(io.Discard, "", 0), func(config.SpeechConfig, *log.Logger) (Engine, error) {
		return engine, nil
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

	engine.results <- Result{ChannelID: "producer", Text: "Go now on one and clear.", Final: true, ReceivedAt: time.Now().UTC()}

	event := waitForSpeechEvent(t, subscription.Events(), EventTypeResultFinal)
	result, ok := event.Payload.(Result)
	if !ok {
		t.Fatalf("payload type = %T, want Result", event.Payload)
	}
	if result.Text != "Go now on one and clear." {
		t.Fatalf("result text = %q, want %q", result.Text, "Go now on one and clear.")
	}
}

type fakeEngine struct {
	results   chan Result
	errors    chan error
	submitted []AudioChunk
	started   bool
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		results: make(chan Result, 8),
		errors:  make(chan error, 8),
	}
}

func (e *fakeEngine) Start(context.Context) error {
	e.started = true
	return nil
}

func (e *fakeEngine) Stop() error {
	if e.started {
		close(e.results)
		close(e.errors)
	}
	return nil
}

func (e *fakeEngine) Submit(chunk AudioChunk) error {
	e.submitted = append(e.submitted, chunk)
	return nil
}

func (e *fakeEngine) Results() <-chan Result {
	return e.results
}

func (e *fakeEngine) Errors() <-chan error {
	return e.errors
}

func waitForSpeechEvent(t *testing.T, eventsCh <-chan events.Event, eventType string) events.Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
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

func waitForSubmittedChunk(t *testing.T, engine *fakeEngine, channelID string) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for submitted chunk channel=%q", channelID)
		case <-ticker.C:
			for _, chunk := range engine.submitted {
				if chunk.ChannelID == channelID {
					return
				}
			}
		}
	}
}
