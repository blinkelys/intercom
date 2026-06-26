package bootstrap

import (
	"fmt"
	"log"

	"procom/internal/app"
	"procom/internal/audio"
	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/frontendbridge"
	"procom/internal/osc"
	"procom/internal/speech"
	"procom/internal/transcript"
)

// Runtime contains the fully wired backend runtime and the frontend bridge bound to it.
type Runtime struct {
	App    *app.App
	Bridge *frontendbridge.Bridge
}

// NewRuntime assembles the backend runtime and desktop bridge using production wiring.
func NewRuntime(logger *log.Logger) (*Runtime, error) {
	runtimeConfig := config.Default()
	persistedChannels, err := config.LoadPersistedChannels()
	if err != nil {
		logger.Printf("load persisted channel settings failed: %v", err)
	} else if len(persistedChannels) > 0 {
		runtimeConfig.Channels = persistedChannels
	}

	var channelManager *channels.Manager
	var audioManager *audio.Manager
	var speechManager *speech.Manager
	var transcriptManager *transcript.Manager
	var oscManager *osc.Manager
	var bridge *frontendbridge.Bridge

	application, err := app.New(
		app.WithConfigSource(config.StaticSource{Config: runtimeConfig}),
		app.WithLogger(logger),
		app.WithComponentFactories(
			func(deps app.Dependencies) (app.Component, error) {
				manager, err := channels.NewManager(deps.Config, deps.Events, deps.Logger)
				if err == nil {
					channelManager = manager
				}
				return manager, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				manager, err := audio.NewManager(deps.Config, deps.Events, deps.Logger, audio.NewDefaultDriver(logger))
				if err == nil {
					audioManager = manager
				}
				return manager, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				manager, err := speech.NewManager(deps.Config, deps.Events, deps.Logger, speech.DefaultEngineFactory)
				if err == nil {
					speechManager = manager
				}
				return manager, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				manager, err := transcript.NewManager(deps.Config, deps.Events, deps.Logger)
				if err == nil {
					transcriptManager = manager
				}
				return manager, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				if transcriptManager == nil {
					return nil, fmt.Errorf("osc dependencies are unavailable")
				}
				manager, err := osc.NewManager(deps.Config, deps.Events, deps.Logger, transcriptManager, nil)
				if err == nil {
					oscManager = manager
				}
				return manager, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				if channelManager == nil || audioManager == nil || oscManager == nil || speechManager == nil || transcriptManager == nil {
					return nil, fmt.Errorf("frontend bridge dependencies are unavailable")
				}
				currentBridge, err := frontendbridge.New(deps.Config, deps.Events, deps.Logger, channelManager, audioManager, oscManager, speechManager, transcriptManager)
				if err == nil {
					bridge = currentBridge
				}
				return currentBridge, err
			},
		),
	)
	if err != nil {
		return nil, err
	}
	if bridge == nil {
		return nil, fmt.Errorf("frontend bridge was not initialized")
	}

	return &Runtime{App: application, Bridge: bridge}, nil
}
