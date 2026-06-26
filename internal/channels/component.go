package channels

import "procom/internal/app"

// ComponentFactory wires the channel manager into the application runtime.
func ComponentFactory() app.ComponentFactory {
	return func(deps app.Dependencies) (app.Component, error) {
		return NewManager(deps.Config, deps.Events, deps.Logger)
	}
}
