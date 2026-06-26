package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"procom/internal/config"
	"procom/internal/events"
)

const (
	defaultEventBuffer    = 128
	defaultShutdownWindow = 5 * time.Second
)

// App coordinates application-wide services and lifecycle.
type App struct {
	config     config.Config
	bus        *events.Bus
	logger     *log.Logger
	components []Component
	shutdown   time.Duration
}

// Component defines one lifecycle-managed application service.
type Component interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// ComponentFactory constructs a component from shared application dependencies.
type ComponentFactory func(Dependencies) (Component, error)

// Dependencies exposes stable shared services to component factories.
type Dependencies struct {
	Config config.Config
	Events *events.Bus
	Logger *log.Logger
}

// Option configures application construction.
type Option func(*Options)

// Options controls application composition.
type Options struct {
	ConfigSource    config.Source
	EventBufferSize int
	Logger          *log.Logger
	ShutdownTimeout time.Duration
	Factories       []ComponentFactory
}

// New constructs the application runtime and its shared dependencies.
func New(opts ...Option) (*App, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	ctx := context.Background()
	cfg, err := options.ConfigSource.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	bus, err := events.NewBus(options.EventBufferSize)
	if err != nil {
		return nil, fmt.Errorf("create event bus: %w", err)
	}

	deps := Dependencies{
		Config: cfg,
		Events: bus,
		Logger: options.Logger,
	}

	components := make([]Component, 0, len(options.Factories))
	for _, factory := range options.Factories {
		component, err := factory(deps)
		if err != nil {
			bus.Close()
			return nil, fmt.Errorf("build component: %w", err)
		}
		components = append(components, component)
	}

	return &App{
		config:     cfg,
		bus:        bus,
		logger:     options.Logger,
		components: components,
		shutdown:   options.ShutdownTimeout,
	}, nil
}

// Run starts the runtime, blocks until cancellation, and then shuts down in reverse order.
func (a *App) Run(ctx context.Context) error {
	started := 0
	for index, component := range a.components {
		a.logger.Printf("starting component=%s", component.Name())
		if err := component.Start(ctx); err != nil {
			rollbackErr := a.stopComponents(context.Background(), index-1)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("start component %q: %w", component.Name(), err), rollbackErr)
			}
			return fmt.Errorf("start component %q: %w", component.Name(), err)
		}
		started++
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdown)
	defer cancel()

	err := a.stopComponents(shutdownCtx, started-1)
	a.bus.Close()
	return err
}

// Config returns the loaded runtime configuration.
func (a *App) Config() config.Config {
	return a.config
}

func defaultOptions() Options {
	return Options{
		ConfigSource:    config.StaticSource{Config: config.Default()},
		EventBufferSize: defaultEventBuffer,
		Logger:          log.New(io.Discard, "", 0),
		ShutdownTimeout: defaultShutdownWindow,
	}
}

func (a *App) stopComponents(ctx context.Context, lastIndex int) error {
	if lastIndex < 0 {
		return nil
	}

	var shutdownErr error
	for index := lastIndex; index >= 0; index-- {
		component := a.components[index]
		a.logger.Printf("stopping component=%s", component.Name())
		if err := component.Stop(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("stop component %q: %w", component.Name(), err))
		}
	}

	return shutdownErr
}

// WithConfigSource overrides the application configuration source.
func WithConfigSource(source config.Source) Option {
	return func(options *Options) {
		options.ConfigSource = source
	}
}

// WithEventBufferSize overrides the event bus subscriber buffer size.
func WithEventBufferSize(size int) Option {
	return func(options *Options) {
		options.EventBufferSize = size
	}
}

// WithLogger overrides the application logger.
func WithLogger(logger *log.Logger) Option {
	return func(options *Options) {
		if logger == nil {
			logger = log.New(io.Discard, "", 0)
		}
		options.Logger = logger
	}
}

// WithShutdownTimeout overrides the graceful shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(options *Options) {
		options.ShutdownTimeout = timeout
	}
}

// WithComponentFactories appends component factories to the runtime.
func WithComponentFactories(factories ...ComponentFactory) Option {
	return func(options *Options) {
		options.Factories = append(options.Factories, factories...)
	}
}
