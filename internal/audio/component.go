package audio

import "procom/internal/app"

// ComponentFactory wires the audio manager into the application runtime.
func ComponentFactory(driver Driver, opts ...ManagerOption) app.ComponentFactory {
	return func(deps app.Dependencies) (app.Component, error) {
		return NewManager(deps.Config, deps.Events, deps.Logger, driver, opts...)
	}
}
