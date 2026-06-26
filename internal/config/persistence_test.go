package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPersistedChannelsRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PROCOM_STATE_DIR", tempDir)

	channels := []ChannelConfig{
		{
			ID:          "producer",
			Name:        "Producer",
			Color:       "#EF4444",
			Icon:        "🎬",
			InputDevice: "wing::ch:1",
			Language:    "en",
			GainDB:      6,
			Enabled:     true,
		},
		{
			ID:          "md",
			Name:        "Musical Director",
			Color:       "#22C55E",
			Icon:        "🎼",
			InputDevice: "wing::ch:2",
			Language:    "no",
			GainDB:      3,
			Enabled:     true,
		},
	}

	if err := SavePersistedChannels(channels); err != nil {
		t.Fatalf("save persisted channels: %v", err)
	}

	loaded, err := LoadPersistedChannels()
	if err != nil {
		t.Fatalf("load persisted channels: %v", err)
	}

	if !reflect.DeepEqual(loaded, channels) {
		t.Fatalf("loaded channels = %#v, want %#v", loaded, channels)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "channels.json")); err != nil {
		t.Fatalf("expected persisted file: %v", err)
	}
}

func TestLoadPersistedChannelsMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PROCOM_STATE_DIR", tempDir)

	loaded, err := LoadPersistedChannels()
	if err != nil {
		t.Fatalf("load persisted channels: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded channels = %#v, want empty", loaded)
	}
}
