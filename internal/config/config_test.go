package config

import (
	"context"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	t.Parallel()

	if err := Default().Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestStaticSourceRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := StaticSource{Config: Config{Channels: []ChannelConfig{{ID: "dup", Name: "One"}, {ID: "dup", Name: "Two"}}}}.Load(context.Background())
	if err == nil {
		t.Fatal("expected duplicate channel ids to fail validation")
	}
}

func TestValidateRejectsEnabledSpeechWithoutWorkerCommand(t *testing.T) {
	t.Parallel()

	cfg := Default()
	cfg.Speech.Enabled = true
	cfg.Speech.WorkerCommand = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected speech config without worker command to fail validation")
	}
}
