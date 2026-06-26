package bootstrap

import (
	"fmt"
	"log"

	"procom/internal/app"
	"procom/internal/audio"
	"procom/internal/channels"
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
	var channelManager *channels.Manager
	var audioManager *audio.Manager
	var speechManager *speech.Manager
	var transcriptManager *transcript.Manager
	var bridge *frontendbridge.Bridge

	application, err := app.New(
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
				if channelManager == nil || audioManager == nil || speechManager == nil || transcriptManager == nil {
					return nil, fmt.Errorf("frontend bridge dependencies are unavailable")
				}
				currentBridge, err := frontendbridge.New(deps.Config, deps.Events, deps.Logger, channelManager, audioManager, speechManager, transcriptManager)
				if err == nil {
					bridge = currentBridge
				}
				return currentBridge, err
			},
			func(deps app.Dependencies) (app.Component, error) {
				if transcriptManager == nil {
					return nil, fmt.Errorf("osc dependencies are unavailable")
				}
				return osc.NewManager(deps.Config, deps.Events, deps.Logger, transcriptManager, nil)
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
