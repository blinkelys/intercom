package osc

import (
	"procom/internal/app"
	"procom/internal/transcript"
)

// ComponentFactory wires the OSC manager into the application runtime.
func ComponentFactory(transcriptManager *transcript.Manager, sender Sender) app.ComponentFactory {
	return func(deps app.Dependencies) (app.Component, error) {
		return NewManager(deps.Config, deps.Events, deps.Logger, transcriptManager, sender)
	}
}
