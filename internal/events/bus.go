package events

import (
	"fmt"
	"sync"
)

// Event represents a domain event passed between subsystems.
type Event struct {
	Type    string
	Payload any
}

// Bus is a buffered fan-out event bus used for low-coupling subsystem communication.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan Event
	nextID      uint64
	bufferSize  int
	closed      bool
}

// Subscription represents one event bus subscription.
type Subscription struct {
	bus  *Bus
	id   uint64
	one  sync.Once
	read <-chan Event
}

// NewBus allocates a buffered event bus.
func NewBus(size int) (*Bus, error) {
	if size <= 0 {
		return nil, fmt.Errorf("buffer size must be positive")
	}

	return &Bus{
		subscribers: make(map[uint64]chan Event),
		bufferSize:  size,
	}, nil
}

// Publish attempts to enqueue an event for each subscriber without blocking callers indefinitely.
func (b *Bus) Publish(event Event) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return false
	}

	delivered := true
	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
			delivered = false
		}
	}

	return delivered
}

// Subscribe registers a new listener and returns a subscription handle.
func (b *Bus) Subscribe() (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, fmt.Errorf("event bus is closed")
	}

	id := b.nextID
	b.nextID++

	channel := make(chan Event, b.bufferSize)
	b.subscribers[id] = channel

	return &Subscription{
		bus:  b,
		id:   id,
		read: channel,
	}, nil
}

// Close shuts down the bus and all subscriber channels.
func (b *Bus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true

	subscribers := make([]chan Event, 0, len(b.subscribers))
	for id, subscriber := range b.subscribers {
		subscribers = append(subscribers, subscriber)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()

	for _, subscriber := range subscribers {
		close(subscriber)
	}
}

// Events returns the read-only event stream for the subscription.
func (s *Subscription) Events() <-chan Event {
	return s.read
}

// Close removes the subscription from the parent bus.
func (s *Subscription) Close() {
	s.one.Do(func() {
		s.bus.unsubscribe(s.id)
	})
}

func (b *Bus) unsubscribe(id uint64) {
	b.mu.Lock()
	subscriber, ok := b.subscribers[id]
	if ok {
		delete(b.subscribers, id)
	}
	b.mu.Unlock()

	if ok {
		close(subscriber)
	}
}
