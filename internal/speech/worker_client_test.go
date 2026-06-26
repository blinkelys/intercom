package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"procom/internal/config"
)

func TestWorkerClientStartSubmitAndStop(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard, "", 0)
	client := NewWorkerClient(config.SpeechConfig{
		Enabled:       true,
		Engine:        "mlx_whisper",
		WorkerCommand: os.Args[0],
		WorkerArgs:    []string{"-test.run=TestSpeechWorkerHelperProcess", "--"},
		StartTimeout:  1000,
		ResultBuffer:  8,
		ErrorBuffer:   8,
	}, logger, helperRunner{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("start client: %v", err)
	}

	if err := client.Submit(AudioChunk{ChannelID: "producer", Language: "en", SampleRate: 16000, Frames: []float32{0.1, 0.2}}); err != nil {
		t.Fatalf("submit chunk: %v", err)
	}

	select {
	case result := <-client.Results():
		if result.ChannelID != "producer" {
			t.Fatalf("result channel = %q, want producer", result.ChannelID)
		}
		if result.Text != "frames:2" {
			t.Fatalf("result text = %q, want %q", result.Text, "frames:2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker result")
	}

	if err := client.Stop(); err != nil {
		t.Fatalf("stop client: %v", err)
	}
}

func TestSpeechWorkerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SPEECH_HELPER_PROCESS") != "1" {
		return
	}

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var request workerRequest
		if err := decoder.Decode(&request); err != nil {
			os.Exit(0)
		}
		switch request.Type {
		case "start":
			_ = encoder.Encode(workerResponse{Type: "ready", Engine: request.Engine})
		case "audio_chunk":
			_ = encoder.Encode(workerResponse{Type: "result", ChannelID: request.ChannelID, Text: fmt.Sprintf("frames:%d", len(request.Frames)), Final: true, Timestamp: time.Now().UTC()})
		case "stop":
			_ = encoder.Encode(workerResponse{Type: "stopped", Engine: request.Engine})
			os.Exit(0)
		}
	}
}

type helperRunner struct{}

func (helperRunner) Start(ctx context.Context, command string, args ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(os.Environ(), "GO_WANT_SPEECH_HELPER_PROCESS=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, err
	}
	return cmd, stdin, stdout, stderr, nil
}
