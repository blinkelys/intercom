package frontendbridge

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"procom/internal/audio"
	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/osc"
	"procom/internal/speech"
	"procom/internal/transcript"
)

func TestBridgeGetBootstrapMapsChannelAndTranscriptState(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(32)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	logger := log.New(io.Discard, "", 0)
	cfg := config.Config{
		Channels: []config.ChannelConfig{{
			ID:       "producer",
			Name:     "Producer",
			Color:    "#EF4444",
			Icon:     "🎬",
			Language: "en",
			Enabled:  true,
		}},
		Keywords: []config.KeywordConfig{{Phrase: "GO", HighlightColor: "#22C55E", WholeWord: true}},
	}

	channelManager, err := channels.NewManager(cfg, bus, logger)
	if err != nil {
		t.Fatalf("new channels manager: %v", err)
	}
	audioManager, err := audio.NewManager(cfg, bus, logger, audio.NullDriver{})
	if err != nil {
		t.Fatalf("new audio manager: %v", err)
	}
	transcriptManager, err := transcript.NewManager(cfg, bus, logger)
	if err != nil {
		t.Fatalf("new transcript manager: %v", err)
	}
	oscManager, err := osc.NewManager(cfg, bus, logger, transcriptManager, nil)
	if err != nil {
		t.Fatalf("new osc manager: %v", err)
	}
	speechManager, err := speech.NewManager(cfg, bus, logger, speech.DefaultEngineFactory)
	if err != nil {
		t.Fatalf("new speech manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := channelManager.Start(ctx); err != nil {
		t.Fatalf("start channels manager: %v", err)
	}
	if err := audioManager.Start(ctx); err != nil {
		t.Fatalf("start audio manager: %v", err)
	}
	if err := transcriptManager.Start(ctx); err != nil {
		t.Fatalf("start transcript manager: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = transcriptManager.Stop(stopCtx)
		_ = audioManager.Stop(stopCtx)
		_ = channelManager.Stop(stopCtx)
	}()

	bus.Publish(events.Event{Type: speech.EventTypeResultFinal, Payload: speech.Result{ChannelID: "producer", Text: "Ready, go now", Final: true, ReceivedAt: time.Now().UTC()}})

	deadline := time.After(time.Second)
	for {
		if len(transcriptManager.Entries()) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for transcript entry")
		default:
		}
	}

	bridge, err := New(cfg, bus, logger, channelManager, audioManager, oscManager, speechManager, transcriptManager)
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}

	payload, err := bridge.GetBootstrap()
	if err != nil {
		t.Fatalf("get bootstrap: %v", err)
	}
	if len(payload.Channels) != 1 {
		t.Fatalf("channels len = %d, want 1", len(payload.Channels))
	}
	if len(payload.Snapshot.Entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(payload.Snapshot.Entries))
	}
	if payload.Speech.Model == "" {
		t.Fatal("speech diagnostics model should be populated")
	}
	if len(payload.Snapshot.Entries[0].Keywords) != 1 || payload.Snapshot.Entries[0].Keywords[0] != "GO" {
		t.Fatalf("keywords = %v, want [GO]", payload.Snapshot.Entries[0].Keywords)
	}
}

func TestBridgeUpdateChannelDelegatesToManager(t *testing.T) {
	t.Parallel()

	bus, err := events.NewBus(8)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	logger := log.New(io.Discard, "", 0)
	cfg := config.Default()

	channelManager, err := channels.NewManager(cfg, bus, logger)
	if err != nil {
		t.Fatalf("new channels manager: %v", err)
	}
	audioManager, err := audio.NewManager(cfg, bus, logger, audio.NullDriver{})
	if err != nil {
		t.Fatalf("new audio manager: %v", err)
	}
	transcriptManager, err := transcript.NewManager(cfg, bus, logger)
	if err != nil {
		t.Fatalf("new transcript manager: %v", err)
	}
	oscManager, err := osc.NewManager(cfg, bus, logger, transcriptManager, nil)
	if err != nil {
		t.Fatalf("new osc manager: %v", err)
	}
	speechManager, err := speech.NewManager(cfg, bus, logger, speech.DefaultEngineFactory)
	if err != nil {
		t.Fatalf("new speech manager: %v", err)
	}

	bridge, err := New(cfg, bus, logger, channelManager, audioManager, oscManager, speechManager, transcriptManager)
	if err != nil {
		t.Fatalf("new bridge: %v", err)
	}

	updated, err := bridge.UpdateChannel(ChannelUpdateInput{
		ID:          "producer",
		Name:        "Calling Producer",
		Color:       "#2563EB",
		Icon:        "🎧",
		InputDevice: "Input A",
		Language:    "no",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if updated.Name != "Calling Producer" {
		t.Fatalf("updated name = %q, want Calling Producer", updated.Name)
	}
}
