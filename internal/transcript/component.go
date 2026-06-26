package transcript

import "procom/internal/app"

// ComponentFactory wires the transcript manager into the application runtime.
func ComponentFactory() app.ComponentFactory {
	return func(deps app.Dependencies) (app.Component, error) {
		return NewManager(deps.Config, deps.Events, deps.Logger)
	}
}
