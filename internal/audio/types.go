package audio

import "time"

const (
	// EventTypeDevicesUpdated is published when the audio device inventory changes.
	EventTypeDevicesUpdated = "audio.devices.updated"
	// EventTypePipelineStateChanged is published when a channel pipeline changes state.
	EventTypePipelineStateChanged = "audio.pipeline.state_changed"
	// EventTypeChunkCaptured is published when a channel captures a chunk of audio.
	EventTypeChunkCaptured = "audio.chunk.captured"
)

// PipelineState describes the current health of one audio pipeline.
type PipelineState string

const (
	PipelineStateDisabled      PipelineState = "disabled"
	PipelineStateUnassigned    PipelineState = "unassigned"
	PipelineStateMissingDevice PipelineState = "missing_device"
	PipelineStateStarting      PipelineState = "starting"
	PipelineStateRunning       PipelineState = "running"
	PipelineStateFailed        PipelineState = "failed"
	PipelineStateStopped       PipelineState = "stopped"
)

// Device describes an audio input device.
type Device struct {
	ID   string
	Name string
}

// DeviceInventory describes the currently available input devices.
type DeviceInventory struct {
	Devices     []Device
	RefreshedAt time.Time
	Error       string
}

// StreamConfig defines the desired capture settings for one channel pipeline.
type StreamConfig struct {
	ChannelID       string
	DeviceID        string
	SampleRate      int
	FramesPerBuffer int
}

// PipelineStatus reports the current health of one channel audio pipeline.
type PipelineStatus struct {
	ChannelID  string
	DeviceID   string
	State      PipelineState
	OccurredAt time.Time
	Message    string
}

// SampleChunk is a frame batch emitted by audio capture.
type SampleChunk struct {
	ChannelID  string
	SampleRate int
	Frames     []float32
	CapturedAt time.Time
}
