package channels

import (
	"context"
	"io"
	"log"
	"strings"
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

func TestManagerAddAndRemoveChannel(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(8)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Default(), bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	created, err := manager.Add(AddRequest{
		ID:       "fx",
		Name:     "Effects",
		Color:    "#0EA5E9",
		Icon:     "🎛",
		Language: "en",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("add channel: %v", err)
	}
	if created.ID != "fx" {
		t.Fatalf("created id = %q, want fx", created.ID)
	}

	if err := manager.Remove("fx"); err != nil {
		t.Fatalf("remove channel: %v", err)
	}

	for _, channel := range manager.Snapshot().Channels {
		if channel.ID == "fx" {
			t.Fatal("expected removed channel to be absent")
		}
	}
}

func TestManagerAddRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(4)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Default(), bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	_, err = manager.Add(AddRequest{
		ID:       "producer-2",
		Name:     "Producer",
		Color:    "#0EA5E9",
		Icon:     "🎛",
		Language: "en",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected duplicate name to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManagerUpdateRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(4)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Default(), bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	_, err = manager.Update(UpdateRequest{
		ID:       "musical-director",
		Name:     "Producer",
		Color:    "#22c55e",
		Icon:     "🎼",
		Language: "en",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected duplicate name to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManagerRemoveRejectsLastChannel(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Channels = []config.ChannelConfig{cfg.Channels[0]}

	bus, err := events.NewBus(4)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(cfg, bus, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	err = manager.Remove(cfg.Channels[0].ID)
	if err == nil {
		t.Fatal("expected removing last channel to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "at least one") {
		t.Fatalf("unexpected error: %v", err)
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
