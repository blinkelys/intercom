package osc

import (
	"context"
	"fmt"
	"log"
	"sync"

	"procom/internal/config"
	"procom/internal/events"
	"procom/internal/transcript"
)

const outboundQueueSize = 64

// Manager sends asynchronous OSC messages for finalized keyword matches.
type Manager struct {
	config     config.Config
	bus        *events.Bus
	logger     *log.Logger
	transcript *transcript.Manager
	sender     Sender
	rules      map[string]config.KeywordConfig

	mu           sync.Mutex
	started      bool
	cancel       context.CancelFunc
	subscription *events.Subscription
	queue        chan Message
	workerDone   chan struct{}
}

// NewManager constructs an OSC manager.
func NewManager(cfg config.Config, bus *events.Bus, logger *log.Logger, transcriptManager *transcript.Manager, sender Sender) (*Manager, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if transcriptManager == nil {
		return nil, fmt.Errorf("transcript manager is required")
	}
	rules := make(map[string]config.KeywordConfig, len(cfg.Keywords))
	for _, rule := range cfg.Keywords {
		rules[rule.Phrase] = rule
	}
	return &Manager{
		config:     cfg,
		bus:        bus,
		logger:     logger,
		transcript: transcriptManager,
		sender:     sender,
		rules:      rules,
	}, nil
}

// Name returns the lifecycle component name.
func (m *Manager) Name() string {
	return "osc-manager"
}

// Start begins listening for finalized transcript updates.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("osc manager already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	subscription, err := m.bus.Subscribe()
	if err != nil {
		cancel()
		m.mu.Unlock()
		return fmt.Errorf("subscribe to event bus: %w", err)
	}
	if m.config.OSC.Enabled && m.sender == nil {
		sender, err := NewUDPClient(m.config.OSC.Destination, m.config.OSC.Port)
		if err != nil {
			subscription.Close()
			cancel()
			m.mu.Unlock()
			return err
		}
		m.sender = sender
	}
	m.cancel = cancel
	m.subscription = subscription
	m.queue = make(chan Message, outboundQueueSize)
	m.workerDone = make(chan struct{})
	m.started = true
	queue := m.queue
	workerDone := m.workerDone
	m.mu.Unlock()

	go m.consume(runCtx, subscription, queue)
	go m.dispatch(runCtx, queue, workerDone)
	return nil
}

// Stop drains background work and detaches the manager.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	cancel := m.cancel
	subscription := m.subscription
	queue := m.queue
	workerDone := m.workerDone
	m.cancel = nil
	m.subscription = nil
	m.queue = nil
	m.workerDone = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if subscription != nil {
		subscription.Close()
	}
	if queue != nil {
		close(queue)
	}
	if workerDone == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-workerDone:
		return nil
	}
}

func (m *Manager) consume(ctx context.Context, subscription *events.Subscription, queue chan<- Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			if event.Type != transcript.EventTypeUpdated {
				continue
			}
			update, ok := event.Payload.(transcript.Update)
			if !ok || update.Kind != "final" || update.EntryID == "" || !m.config.OSC.Enabled {
				continue
			}
			entry, ok := m.transcript.Entry(update.EntryID)
			if !ok {
				continue
			}
			for _, keyword := range entry.Keywords {
				rule, exists := m.rules[keyword]
				if !exists || !rule.TriggerEnabled || rule.OSCAddress == "" {
					continue
				}
				message := Message{Address: rule.OSCAddress, Arguments: append([]string(nil), rule.OSCArguments...)}
				select {
				case queue <- message:
				default:
					m.logger.Printf("osc queue full, dropping message address=%s", message.Address)
				}
			}
		}
	}
}

func (m *Manager) dispatch(ctx context.Context, queue <-chan Message, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-queue:
			if !ok {
				return
			}
			if m.sender == nil {
				continue
			}
			if err := m.sender.Send(message); err != nil {
				m.logger.Printf("osc send failed address=%s: %v", message.Address, err)
			}
		}
	}
}
