package osc

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/speech"
	"procom/internal/transcript"
)

func TestManagerSendsOSCForTriggeredKeywords(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	logger := log.New(io.Discard, "", 0)
	cfg := config.Config{
		Channels: []config.ChannelConfig{{ID: "producer", Name: "Producer", Color: "#EF4444", Icon: "🎬", Language: "en", Enabled: true}},
		Keywords: []config.KeywordConfig{{
			Phrase:         "GO",
			HighlightColor: "#22C55E",
			WholeWord:      true,
			TriggerEnabled: true,
			OSCAddress:     "/go",
			OSCArguments:   []string{"producer"},
		}},
		OSC: config.OSCConfig{Enabled: true, Destination: "127.0.0.1", Port: 8000},
	}

	transcriptManager, err := transcript.NewManager(cfg, bus, logger)
	if err != nil {
		t.Fatalf("new transcript manager: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := transcriptManager.Start(ctx); err != nil {
		t.Fatalf("start transcript manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = transcriptManager.Stop(stopCtx)
	}()

	sender := &fakeSender{sent: make(chan Message, 1)}
	manager, err := NewManager(cfg, bus, logger, transcriptManager, sender)
	if err != nil {
		t.Fatalf("new osc manager: %v", err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start osc manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = manager.Stop(stopCtx)
	}()

	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "producer", Text: "Go now", Final: true, ReceivedAt: time.Now().UTC()}})

	select {
	case message := <-sender.sent:
		if message.Address != "/go" {
			t.Fatalf("message address = %q, want /go", message.Address)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for osc message")
	}
}

type fakeSender struct {
	sent chan Message
}

func (s *fakeSender) Send(message Message) error {
	s.sent <- message
	return nil
}
