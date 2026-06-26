package speech

import "time"

type workerRequest struct {
	Type       string    `json:"type"`
	Engine     string    `json:"engine,omitempty"`
	ChannelID  string    `json:"channelId,omitempty"`
	Language   string    `json:"language,omitempty"`
	Prompt     string    `json:"prompt,omitempty"`
	SampleRate int       `json:"sampleRate,omitempty"`
	Frames     []float32 `json:"frames,omitempty"`
	CapturedAt time.Time `json:"capturedAt,omitempty"`
}

type workerResponse struct {
	Type        string    `json:"type"`
	Engine      string    `json:"engine,omitempty"`
	ChannelID   string    `json:"channelId,omitempty"`
	Language    string    `json:"language,omitempty"`
	Model       string    `json:"model,omitempty"`
	Task        string    `json:"task,omitempty"`
	InferenceMS int       `json:"inferenceMs,omitempty"`
	Text        string    `json:"text,omitempty"`
	Final       bool      `json:"final,omitempty"`
	Message     string    `json:"message,omitempty"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
}
