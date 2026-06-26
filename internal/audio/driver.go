package audio

import (
	"context"
	"errors"
)

var errStreamUnsupported = errors.New("audio stream opening is not supported by this driver")

// Driver abstracts platform-specific audio device enumeration and capture.
type Driver interface {
	Devices(context.Context) ([]Device, error)
	OpenStream(context.Context, StreamConfig) (Stream, error)
}

// Stream represents one active input capture stream.
type Stream interface {
	Start(context.Context) error
	Chunks() <-chan SampleChunk
	Stop(context.Context) error
}

// NullDriver is a safe fallback driver that exposes no devices.
type NullDriver struct{}

// Devices returns an empty inventory.
func (NullDriver) Devices(context.Context) ([]Device, error) {
	return nil, nil
}

// OpenStream always fails because the null driver does not capture audio.
func (NullDriver) OpenStream(context.Context, StreamConfig) (Stream, error) {
	return nil, errStreamUnsupported
}
