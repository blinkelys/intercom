package channels

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"procom/internal/config"
	"procom/internal/events"
)

func TestManagerUpdateMutatesChannelAndPublishesEvent(t *testing.T) {
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

	manager, err := NewManager(config.Default(), bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	updated, err := manager.Update(UpdateRequest{
		ID:          "producer",
		Name:        "Calling Producer",
		Color:       "#2563EB",
		Icon:        "🎧",
		InputDevice: "input-1",
		Language:    "no",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("update manager: %v", err)
	}
	if updated.Name != "Calling Producer" {
		t.Fatalf("updated name = %q, want Calling Producer", updated.Name)
	}

	event := waitForChannelEvent(t, subscription.Events())
	payload, ok := event.Payload.(Update)
	if !ok {
		t.Fatalf("payload type = %T, want Update", event.Payload)
	}
	if payload.Channel.Color != "#2563EB" {
		t.Fatalf("payload color = %q, want #2563EB", payload.Channel.Color)
	}

	snapshot := manager.Snapshot()
	if snapshot.Channels[0].Icon != "🎧" {
		t.Fatalf("snapshot icon = %q, want 🎧", snapshot.Channels[0].Icon)
	}
}

func TestManagerUpdateRejectsInvalidChannel(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(4)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Default(), bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	_, err = manager.Update(UpdateRequest{ID: "producer", Name: "", Color: "#fff", Icon: "🎬", Language: "en", Enabled: true})
	if err == nil {
		t.Fatal("expected invalid update to fail")
	}
}

func waitForChannelEvent(t *testing.T, eventsCh <-chan events.Event) events.Event {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for channel update event")
		case event := <-eventsCh:
			if event.Type == EventTypeUpdated {
				return event
			}
		}
	}
}
