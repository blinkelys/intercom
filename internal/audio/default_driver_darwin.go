//go:build darwin

package audio

import "log"

// NewDefaultDriver returns the primary macOS audio driver.
func NewDefaultDriver(logger *log.Logger) Driver {
	return NewMacOSDriver(logger)
}
