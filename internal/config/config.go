package config

import (
	"fmt"
	"strings"
)

// Config contains top-level runtime configuration.
type Config struct {
	Channels []ChannelConfig `json:"channels"`
	Keywords []KeywordConfig `json:"keywords"`
	Speech   SpeechConfig    `json:"speech"`
	OSC      OSCConfig       `json:"osc"`
}

// SpeechConfig defines speech engine runtime settings.
type SpeechConfig struct {
	Enabled       bool     `json:"enabled"`
	Engine        string   `json:"engine"`
	WorkerCommand string   `json:"workerCommand"`
	WorkerArgs    []string `json:"workerArgs"`
	StartTimeout  int      `json:"startTimeoutMs"`
	ResultBuffer  int      `json:"resultBuffer"`
	ErrorBuffer   int      `json:"errorBuffer"`
}

// ChannelConfig defines one communication channel.
type ChannelConfig struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Icon        string  `json:"icon"`
	InputDevice string  `json:"inputDevice"`
	Language    string  `json:"language"`
	GainDB      float64 `json:"gainDb"`
	Enabled     bool    `json:"enabled"`
}

// KeywordConfig defines one keyword rule.
type KeywordConfig struct {
	Phrase          string   `json:"phrase"`
	HighlightColor  string   `json:"highlightColor"`
	CaseSensitive   bool     `json:"caseSensitive"`
	WholeWord       bool     `json:"wholeWord"`
	OSCAddress      string   `json:"oscAddress"`
	OSCArguments    []string `json:"oscArguments"`
	TriggerEnabled  bool     `json:"triggerEnabled"`
	HighlightEnable bool     `json:"highlightEnable"`
}

// OSCConfig defines global OSC settings.
type OSCConfig struct {
	Enabled     bool   `json:"enabled"`
	Destination string `json:"destination"`
	Port        int    `json:"port"`
}

// Default returns a conservative runtime baseline.
func Default() Config {
	return Config{
		Channels: []ChannelConfig{
			{
				ID:       "producer",
				Name:     "Producer",
				Color:    "#EF4444",
				Icon:     "🎬",
				Language: "en",
				Enabled:  true,
			},
			{
				ID:       "musical-director",
				Name:     "Musical Director",
				Color:    "#22C55E",
				Icon:     "🎼",
				Language: "en",
				Enabled:  true,
			},
		},
		Speech: SpeechConfig{
			Enabled:       true,
			Engine:        "mlx_whisper",
			WorkerCommand: "python3",
			StartTimeout:  3000,
			ResultBuffer:  128,
			ErrorBuffer:   32,
		},
	}
}

// Validate ensures the configuration is internally consistent.
func (c Config) Validate() error {
	if len(c.Channels) == 0 {
		return fmt.Errorf("at least one channel is required")
	}
	if len(c.Channels) > 8 {
		return fmt.Errorf("a maximum of 8 channels is supported")
	}

	seenChannelIDs := make(map[string]struct{}, len(c.Channels))
	for _, channel := range c.Channels {
		if strings.TrimSpace(channel.ID) == "" {
			return fmt.Errorf("channel id is required")
		}
		if strings.TrimSpace(channel.Name) == "" {
			return fmt.Errorf("channel %q name is required", channel.ID)
		}
		if _, exists := seenChannelIDs[channel.ID]; exists {
			return fmt.Errorf("duplicate channel id %q", channel.ID)
		}
		if channel.GainDB < -24 || channel.GainDB > 36 {
			return fmt.Errorf("channel %q gainDb %.2f is out of range (-24..36)", channel.ID, channel.GainDB)
		}
		seenChannelIDs[channel.ID] = struct{}{}
	}

	for _, keyword := range c.Keywords {
		if strings.TrimSpace(keyword.Phrase) == "" {
			return fmt.Errorf("keyword phrase is required")
		}
	}

	if c.Speech.StartTimeout < 0 {
		return fmt.Errorf("speech start timeout must not be negative")
	}
	if c.Speech.ResultBuffer < 0 {
		return fmt.Errorf("speech result buffer must not be negative")
	}
	if c.Speech.ErrorBuffer < 0 {
		return fmt.Errorf("speech error buffer must not be negative")
	}
	if c.Speech.Enabled {
		engine := strings.TrimSpace(c.Speech.Engine)
		if engine == "" {
			return fmt.Errorf("speech engine is required when speech is enabled")
		}
		if engine != "mlx_whisper" {
			return fmt.Errorf("unsupported speech engine %q", c.Speech.Engine)
		}
		if strings.TrimSpace(c.Speech.WorkerCommand) == "" {
			return fmt.Errorf("speech worker command is required when speech is enabled")
		}
	}

	if c.OSC.Port < 0 || c.OSC.Port > 65535 {
		return fmt.Errorf("osc port %d is out of range", c.OSC.Port)
	}

	if c.OSC.Enabled && strings.TrimSpace(c.OSC.Destination) == "" {
		return fmt.Errorf("osc destination is required when osc is enabled")
	}

	return nil
}
