//go:build !darwin

package audio

import "log"

// NewDefaultDriver returns a safe fallback driver on unsupported platforms.
func NewDefaultDriver(*log.Logger) Driver {
	return NullDriver{}
}
