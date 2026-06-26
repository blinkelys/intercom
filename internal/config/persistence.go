package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	channelStateFileName = "channels.json"
	stateDirEnvVar       = "PROCOM_STATE_DIR"
)

type channelStateFile struct {
	Channels []ChannelConfig `json:"channels"`
}

// LoadPersistedChannels loads channel settings saved from prior runtime updates.
func LoadPersistedChannels() ([]ChannelConfig, error) {
	path, err := persistedChannelsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read persisted channels: %w", err)
	}

	var payload channelStateFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode persisted channels: %w", err)
	}

	normalized := normalizeChannels(payload.Channels)
	if len(normalized) == 0 {
		return nil, nil
	}
	if err := (Config{Channels: normalized}).Validate(); err != nil {
		return nil, fmt.Errorf("validate persisted channels: %w", err)
	}

	return normalized, nil
}

// SavePersistedChannels writes channel settings to disk so they survive app restarts.
func SavePersistedChannels(channels []ChannelConfig) error {
	normalized := normalizeChannels(channels)
	if err := (Config{Channels: normalized}).Validate(); err != nil {
		return fmt.Errorf("validate channels before persist: %w", err)
	}

	path, err := persistedChannelsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create persistence directory: %w", err)
	}

	payload := channelStateFile{Channels: normalized}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode persisted channels: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write persisted channels temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename persisted channels file: %w", err)
	}

	return nil
}

func normalizeChannels(channels []ChannelConfig) []ChannelConfig {
	normalized := make([]ChannelConfig, 0, len(channels))
	for _, channel := range channels {
		normalized = append(normalized, ChannelConfig{
			ID:          strings.TrimSpace(channel.ID),
			Name:        strings.TrimSpace(channel.Name),
			Color:       strings.TrimSpace(channel.Color),
			Icon:        channel.Icon,
			InputDevice: strings.TrimSpace(channel.InputDevice),
			Language:    strings.TrimSpace(channel.Language),
			GainDB:      channel.GainDB,
			Enabled:     channel.Enabled,
		})
	}
	return normalized
}

func persistedChannelsPath() (string, error) {
	baseDir := strings.TrimSpace(os.Getenv(stateDirEnvVar))
	if baseDir == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		baseDir = filepath.Join(userConfigDir, "procom")
	}

	return filepath.Join(baseDir, channelStateFileName), nil
}
