package speech

import (
	"log"

	"procom/internal/app"
	"procom/internal/config"
)

// ComponentFactory wires the speech manager into the application runtime.
func ComponentFactory(factory EngineFactory) app.ComponentFactory {
	return func(deps app.Dependencies) (app.Component, error) {
		return NewManager(deps.Config, deps.Events, deps.Logger, factory)
	}
}

// DefaultEngineFactory creates the default MLX Whisper worker-backed engine.
func DefaultEngineFactory(cfg config.SpeechConfig, logger *log.Logger) (Engine, error) {
	return NewMLXWhisperEngine(cfg, logger), nil
}
