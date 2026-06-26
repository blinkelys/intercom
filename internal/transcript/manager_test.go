package transcript

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/speech"
)

func TestManagerTracksPartialAndFinalResults(t *testing.T) {
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

	manager, err := NewManager(config.Config{Channels: []config.ChannelConfig{{ID: "producer", Name: "Producer"}}}, bus, log.New(io.Discard, "", 0))
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

	partialTime := time.Now().UTC()
	bus.Publish(events.Event{Type: speech.EventTypeResultPartial, Payload: speech.Result{ChannelID: "producer", Text: "stand", ReceivedAt: partialTime}})
	update := waitForTranscriptUpdate(t, subscription.Events(), "partial")
	if update.ChannelID != "producer" {
		t.Fatalf("partial update channel = %q, want producer", update.ChannelID)
	}

	partials := manager.Partials()
	partial, ok := partials["producer"]
	if !ok {
		t.Fatal("expected producer partial transcript")
	}
	if partial.Text != "stand" {
		t.Fatalf("partial text = %q, want stand", partial.Text)
	}

	finalTime := partialTime.Add(2 * time.Second)
	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "producer", Text: "stand by go", Final: true, ReceivedAt: finalTime}})
	finalUpdate := waitForTranscriptUpdate(t, subscription.Events(), "final")
	if finalUpdate.EntryID == "" {
		t.Fatal("expected final update to include entry id")
	}

	snapshot := manager.Snapshot()
	if len(snapshot.Entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(snapshot.Entries))
	}
	if snapshot.Entries[0].ChannelName != "Producer" {
		t.Fatalf("entry channel name = %q, want Producer", snapshot.Entries[0].ChannelName)
	}
	if snapshot.Entries[0].Text != "stand by go" {
		t.Fatalf("entry text = %q, want stand by go", snapshot.Entries[0].Text)
	}
	if len(snapshot.Partials) != 0 {
		t.Fatalf("partials len = %d, want 0", len(snapshot.Partials))
	}
}

func TestManagerClearsPartialOnEmptyPartialText(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Config{Channels: []config.ChannelConfig{{ID: "producer", Name: "Producer"}}}, bus, log.New(io.Discard, "", 0))
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

	bustime := time.Now().UTC()
	bus.Publish(events.Event{Type: speech.EventTypeResultPartial, Payload: speech.Result{ChannelID: "producer", Text: "stand by", ReceivedAt: bustime}})

	deadline := time.After(time.Second)
	for {
		if _, ok := manager.Partials()["producer"]; ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for initial partial")
		default:
		}
	}

	bus.Publish(events.Event{Type: speech.EventTypeResultPartial, Payload: speech.Result{ChannelID: "producer", Text: "", ReceivedAt: bustime.Add(time.Second)}})

	deadline = time.After(time.Second)
	for {
		if _, ok := manager.Partials()["producer"]; !ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for partial clear")
		default:
		}
	}
}

func TestManagerAddsKeywordMetadataToFinalEntries(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Config{
		Channels: []config.ChannelConfig{{ID: "producer", Name: "Producer"}},
		Keywords: []config.KeywordConfig{{
			Phrase:         "GO",
			HighlightColor: "#22C55E",
			WholeWord:      true,
		}},
	}, bus, log.New(io.Discard, "", 0))
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

	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "producer", Text: "Ready, go now", Final: true, ReceivedAt: time.Now().UTC()}})

	deadline := time.After(time.Second)
	for {
		entries := manager.Entries()
		if len(entries) == 1 {
			if len(entries[0].Keywords) != 1 || entries[0].Keywords[0] != "GO" {
				t.Fatalf("keywords = %v, want [GO]", entries[0].Keywords)
			}
			if len(entries[0].Highlights) != 1 {
				t.Fatalf("highlights len = %d, want 1", len(entries[0].Highlights))
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for keyword-enriched transcript entry")
		default:
		}
	}
}

func TestManagerOrdersFinalEntriesChronologically(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(16)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	manager, err := NewManager(config.Config{Channels: []config.ChannelConfig{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}}, bus, log.New(io.Discard, "", 0))
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

	now := time.Now().UTC()
	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "b", Text: "second", Final: true, ReceivedAt: now.Add(2 * time.Second)}})
	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "a", Text: "first", Final: true, ReceivedAt: now}})

	deadline := time.After(time.Second)
	for {
		if len(manager.Entries()) == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for transcript entries")
		default:
		}
	}

	entries := manager.Entries()
	if entries[0].Text != "first" || entries[1].Text != "second" {
		t.Fatalf("entry order = [%q %q], want [first second]", entries[0].Text, entries[1].Text)
	}
}

func waitForTranscriptUpdate(t *testing.T, eventsCh <-chan events.Event, kind string) Update {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for transcript update kind %q", kind)
		case event := <-eventsCh:
			if event.Type != EventTypeUpdated {
				continue
			}
			update, ok := event.Payload.(Update)
			if !ok {
				t.Fatalf("payload type = %T, want Update", event.Payload)
			}
			if update.Kind == kind {
				return update
			}
		}
	}
}
