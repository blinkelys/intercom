package speech

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"procom/internal/config"
)

var errEngineQueueFull = errors.New("speech engine queue is full")

// CommandRunner starts an external worker process.
type CommandRunner interface {
	Start(context.Context, string, ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser, error)
}

type execRunner struct{}

func (execRunner) Start(ctx context.Context, command string, args ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create worker stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create worker stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("start worker process: %w", err)
	}
	return cmd, stdin, stdout, stderr, nil
}

// WorkerClient is a supervised speech engine that communicates with a worker process over JSON Lines.
type WorkerClient struct {
	config config.SpeechConfig
	logger *log.Logger
	runner CommandRunner

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	cmd     *exec.Cmd
	stdin   io.WriteCloser

	requests chan workerRequest
	results  chan Result
	errors   chan error
	ready    chan error
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewMLXWhisperEngine constructs the default speech engine implementation for Apple Silicon.
func NewMLXWhisperEngine(cfg config.SpeechConfig, logger *log.Logger) *WorkerClient {
	if cfg.WorkerCommand == "" {
		cfg.WorkerCommand = "python3"
	}
	helperPath, err := materializeMLXWorkerScript()
	if err == nil {
		cfg.WorkerArgs = append([]string{helperPath}, cfg.WorkerArgs...)
	}
	return NewWorkerClient(cfg, logger, execRunner{})
}

// NewWorkerClient constructs a worker-backed speech engine.
func NewWorkerClient(cfg config.SpeechConfig, logger *log.Logger, runner CommandRunner) *WorkerClient {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if runner == nil {
		runner = execRunner{}
	}
	resultBuffer := cfg.ResultBuffer
	if resultBuffer <= 0 {
		resultBuffer = 128
	}
	errorBuffer := cfg.ErrorBuffer
	if errorBuffer <= 0 {
		errorBuffer = 32
	}
	return &WorkerClient{
		config:   cfg,
		logger:   logger,
		runner:   runner,
		requests: make(chan workerRequest, 512),
		results:  make(chan Result, resultBuffer),
		errors:   make(chan error, errorBuffer),
	}
}

// Start launches the worker and waits for its ready handshake.
func (c *WorkerClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("speech engine already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	cmd, stdin, stdout, stderr, err := c.runner.Start(runCtx, c.config.WorkerCommand, c.config.WorkerArgs...)
	if err != nil {
		cancel()
		c.mu.Unlock()
		return err
	}
	c.cancel = cancel
	c.cmd = cmd
	c.stdin = stdin
	c.ready = make(chan error, 1)
	c.done = make(chan struct{})
	c.started = true
	c.wg.Add(4)
	go c.writeLoop()
	go c.readLoop(stdout)
	go c.stderrLoop(stderr)
	go c.waitLoop()
	c.mu.Unlock()

	if err := c.enqueue(workerRequest{Type: "start", Engine: c.config.Engine}); err != nil {
		_ = c.Stop()
		return err
	}

	timeout := 3 * time.Second
	if c.config.StartTimeout > 0 {
		timeout = time.Duration(c.config.StartTimeout) * time.Millisecond
	}

	select {
	case err := <-c.ready:
		if err != nil {
			_ = c.Stop()
			return err
		}
		return nil
	case <-time.After(timeout):
		_ = c.Stop()
		return fmt.Errorf("speech worker ready timeout after %s", timeout)
	case <-ctx.Done():
		_ = c.Stop()
		return ctx.Err()
	}
}

// Stop terminates the worker process and closes its lifecycle.
func (c *WorkerClient) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = false
	cancel := c.cancel
	stdin := c.stdin
	ready := c.ready
	done := c.done
	c.mu.Unlock()

	_ = c.enqueue(workerRequest{Type: "stop", Engine: c.config.Engine})
	if stdin != nil {
		_ = stdin.Close()
	}
	if cancel != nil {
		cancel()
	}
	if ready != nil {
		select {
		case ready <- nil:
		default:
		}
	}
	if done != nil {
		<-done
	}
	return nil
}

// Submit queues a chunk for worker inference.
func (c *WorkerClient) Submit(chunk AudioChunk) error {
	request := workerRequest{
		Type:       "audio_chunk",
		Engine:     c.config.Engine,
		ChannelID:  chunk.ChannelID,
		Language:   chunk.Language,
		Prompt:     chunk.Prompt,
		SampleRate: chunk.SampleRate,
		Frames:     append([]float32(nil), chunk.Frames...),
		CapturedAt: chunk.CapturedAt,
	}
	return c.enqueue(request)
}

// Results returns the worker result stream.
func (c *WorkerClient) Results() <-chan Result {
	return c.results
}

// Errors returns the worker error stream.
func (c *WorkerClient) Errors() <-chan error {
	return c.errors
}

func (c *WorkerClient) enqueue(request workerRequest) error {
	c.mu.Lock()
	started := c.started
	c.mu.Unlock()
	if !started {
		return fmt.Errorf("speech engine is not running")
	}

	select {
	case c.requests <- request:
		return nil
	default:
		return errEngineQueueFull
	}
}

func (c *WorkerClient) writeLoop() {
	defer c.wg.Done()
	encoder := json.NewEncoder(c.stdin)
	for request := range c.requests {
		if err := encoder.Encode(request); err != nil {
			c.publishError(fmt.Errorf("write worker request: %w", err))
			return
		}
	}
}

func (c *WorkerClient) readLoop(stdout io.Reader) {
	defer c.wg.Done()
	decoder := json.NewDecoder(bufio.NewReader(stdout))
	for {
		var response workerResponse
		if err := decoder.Decode(&response); err != nil {
			if !errors.Is(err, io.EOF) {
				c.publishError(fmt.Errorf("read worker response: %w", err))
			}
			return
		}
		switch response.Type {
		case "ready":
			c.logger.Printf("speech worker ready engine=%s", response.Engine)
			select {
			case c.ready <- nil:
			default:
			}
		case "result":
			result := Result{
				ChannelID:   response.ChannelID,
				Language:    response.Language,
				Model:       response.Model,
				Task:        response.Task,
				InferenceMS: response.InferenceMS,
				Text:        response.Text,
				Final:       response.Final,
				ReceivedAt:  response.Timestamp,
			}
			if result.ReceivedAt.IsZero() {
				result.ReceivedAt = time.Now().UTC()
			}
			c.logger.Printf("speech worker result channel=%s final=%t chars=%d", result.ChannelID, result.Final, len(result.Text))
			select {
			case c.results <- result:
			default:
				c.publishError(fmt.Errorf("speech result buffer is full"))
			}
		case "error":
			message := response.Message
			if message == "" {
				message = "unknown speech worker error"
			}
			c.publishError(fmt.Errorf(message))
		case "stopped":
			return
		default:
			c.publishError(fmt.Errorf("unsupported worker response type %q", response.Type))
		}
	}
}

func (c *WorkerClient) stderrLoop(stderr io.Reader) {
	defer c.wg.Done()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		c.logger.Printf("speech worker stderr: %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		c.publishError(fmt.Errorf("read worker stderr: %w", err))
	}
}

func (c *WorkerClient) waitLoop() {
	defer c.wg.Done()
	defer close(c.done)
	defer close(c.results)
	defer close(c.errors)
	defer close(c.requests)

	if err := c.cmd.Wait(); err != nil {
		c.publishError(fmt.Errorf("speech worker exited: %w", err))
	}
}

func (c *WorkerClient) publishError(err error) {
	if err == nil {
		return
	}
	select {
	case c.errors <- err:
	default:
		c.logger.Printf("speech worker error dropped: %v", err)
	}
	select {
	case c.ready <- err:
	default:
	}
}
