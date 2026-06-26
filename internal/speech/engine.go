package speech

import (
	"context"
	"time"
)

const (
	// EventTypeEngineStateChanged is published when the speech engine state changes.
	EventTypeEngineStateChanged = "speech.engine.state_changed"
	// EventTypeResultPartial is published when the engine emits an interim transcript.
	EventTypeResultPartial = "speech.result.partial"
	// EventTypeResultFinal is published when the engine emits a finalized transcript.
	EventTypeResultFinal = "speech.result.final"
)

// EngineState describes the current health of the speech subsystem.
type EngineState string

const (
	EngineStateDisabled EngineState = "disabled"
	EngineStateStarting EngineState = "starting"
	EngineStateRunning  EngineState = "running"
	EngineStateFailed   EngineState = "failed"
	EngineStateStopped  EngineState = "stopped"
)

// Status represents one speech engine status update.
type Status struct {
	Engine     string
	State      EngineState
	OccurredAt time.Time
	Message    string
}

// Result represents one recognition update.
type Result struct {
	ChannelID   string
	Language    string
	Model       string
	Task        string
	InferenceMS int
	Text        string
	Final       bool
	ReceivedAt  time.Time
}

// AudioChunk is the input type accepted by speech engines.
type AudioChunk struct {
	ChannelID  string
	Language   string
	Prompt     string
	SampleRate int
	Frames     []float32
	CapturedAt time.Time
}

// Engine defines the stable transcription contract for all speech backends.
type Engine interface {
	Start(ctx context.Context) error
	Stop() error
	Submit(AudioChunk) error
	Results() <-chan Result
	Errors() <-chan error
}
