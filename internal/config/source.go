package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// Source loads runtime configuration from an external or in-memory source.
type Source interface {
	Load(context.Context) (Config, error)
}

// StaticSource returns a fixed in-memory configuration.
type StaticSource struct {
	Config Config
}

// Load returns the embedded configuration after validation.
func (s StaticSource) Load(ctx context.Context) (Config, error) {
	select {
	case <-ctx.Done():
		return Config{}, ctx.Err()
	default:
	}

	if err := s.Config.Validate(); err != nil {
		return Config{}, err
	}

	return s.Config, nil
}

// FileSource loads configuration from a JSON file.
type FileSource struct {
	Path string
}

// Load reads and validates configuration from disk.
func (s FileSource) Load(ctx context.Context) (Config, error) {
	select {
	case <-ctx.Done():
		return Config{}, ctx.Err()
	default:
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
