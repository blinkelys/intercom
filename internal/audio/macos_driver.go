//go:build darwin

package audio

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

//go:embed helpers/apple_audio_helper.swift
var macOSAudioHelperSource string

type helperDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type helperChunk struct {
	SampleRate int       `json:"sampleRate"`
	Frames     []float32 `json:"frames"`
	CapturedAt string    `json:"capturedAt"`
}

// MacOSDriver captures audio through an embedded Swift helper using AVFoundation.
type MacOSDriver struct {
	logger     *log.Logger
	helperOnce sync.Once
	helperPath string
	helperErr  error
}

// NewMacOSDriver constructs the primary macOS audio driver.
func NewMacOSDriver(logger *log.Logger) *MacOSDriver {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &MacOSDriver{logger: logger}
}

// Devices enumerates available audio input devices through the helper.
func (d *MacOSDriver) Devices(ctx context.Context) ([]Device, error) {
	helperPath, err := d.materializeHelper()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "xcrun", "swift", helperPath, "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("enumerate audio devices: %w", err)
	}
	var devices []helperDevice
	if err := json.Unmarshal(output, &devices); err != nil {
		return nil, fmt.Errorf("decode audio devices: %w", err)
	}
	result := make([]Device, 0, len(devices))
	for _, device := range devices {
		result = append(result, Device{ID: device.ID, Name: device.Name})
	}
	return result, nil
}

// OpenStream starts a helper-backed audio capture stream.
func (d *MacOSDriver) OpenStream(_ context.Context, cfg StreamConfig) (Stream, error) {
	helperPath, err := d.materializeHelper()
	if err != nil {
		return nil, err
	}
	return &macOSStream{
		logger:     d.logger,
		helperPath: helperPath,
		config:     cfg,
		chunks:     make(chan SampleChunk, 16),
	}, nil
}

func (d *MacOSDriver) materializeHelper() (string, error) {
	d.helperOnce.Do(func() {
		baseDir, err := os.UserCacheDir()
		if err != nil {
			baseDir = os.TempDir()
		}
		helperDir := filepath.Join(baseDir, "procom", "helpers")
		if err := os.MkdirAll(helperDir, 0o755); err != nil {
			d.helperErr = fmt.Errorf("create audio helper directory: %w", err)
			return
		}
		path := filepath.Join(helperDir, "apple_audio_helper.swift")
		if err := os.WriteFile(path, []byte(macOSAudioHelperSource), 0o644); err != nil {
			d.helperErr = fmt.Errorf("write audio helper source: %w", err)
			return
		}
		d.helperPath = path
	})
	if d.helperErr != nil {
		return "", d.helperErr
	}
	return d.helperPath, nil
}

type macOSStream struct {
	logger     *log.Logger
	helperPath string
	config     StreamConfig
	chunks     chan SampleChunk

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	stdin   io.WriteCloser
	started bool
	done    chan struct{}
	stop    context.CancelFunc
}

func (s *macOSStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("macos stream already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(
		runCtx,
		"xcrun",
		"swift",
		s.helperPath,
		"capture",
		s.config.DeviceID,
		fmt.Sprintf("%d", s.config.SampleRate),
		fmt.Sprintf("%d", s.config.FramesPerBuffer),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("create audio stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("create audio stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("create audio stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start audio helper: %w", err)
	}

	s.cmd = cmd
	s.stdout = stdout
	s.stderr = stderr
	s.stdin = stdin
	s.started = true
	s.stop = cancel
	s.done = make(chan struct{})
	go s.readStdout()
	go s.readStderr()
	go s.wait()
	return nil
}

func (s *macOSStream) Chunks() <-chan SampleChunk {
	return s.chunks
}

func (s *macOSStream) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	stdin := s.stdin
	stop := s.stop
	done := s.done
	s.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if stop != nil {
		stop()
	}
	if done == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *macOSStream) readStdout() {
	decoder := json.NewDecoder(bufio.NewReader(s.stdout))
	for {
		var chunk helperChunk
		if err := decoder.Decode(&chunk); err != nil {
			if err != io.EOF {
				s.logger.Printf("audio helper stdout decode failed: %v", err)
			}
			return
		}
		capturedAt := time.Now().UTC()
		if chunk.CapturedAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, chunk.CapturedAt); err == nil {
				capturedAt = parsed
			}
		}
		s.chunks <- SampleChunk{
			ChannelID:  s.config.ChannelID,
			SampleRate: chunk.SampleRate,
			Frames:     chunk.Frames,
			CapturedAt: capturedAt,
		}
	}
}

func (s *macOSStream) readStderr() {
	scanner := bufio.NewScanner(s.stderr)
	for scanner.Scan() {
		s.logger.Printf("audio helper stderr: %s", scanner.Text())
	}
}

func (s *macOSStream) wait() {
	_ = s.cmd.Wait()
	close(s.chunks)
	close(s.done)
}
