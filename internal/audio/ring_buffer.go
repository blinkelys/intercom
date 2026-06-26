package audio

import (
	"fmt"
	"sync"
)

// RingBuffer stores a fixed-size rolling window of audio frames.
type RingBuffer struct {
	mu    sync.RWMutex
	data  []float32
	head  int
	size  int
	limit int
}

// NewRingBuffer allocates a new fixed-capacity ring buffer.
func NewRingBuffer(capacity int) (*RingBuffer, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("ring buffer capacity must be positive")
	}

	return &RingBuffer{
		data:  make([]float32, capacity),
		limit: capacity,
	}, nil
}

// Write appends frames to the rolling window and overwrites the oldest data when full.
func (b *RingBuffer) Write(frames []float32) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(frames) == 0 {
		return 0
	}

	if len(frames) >= b.limit {
		copy(b.data, frames[len(frames)-b.limit:])
		b.head = 0
		b.size = b.limit
		return len(frames)
	}

	for _, frame := range frames {
		writeIndex := (b.head + b.size) % b.limit
		b.data[writeIndex] = frame
		if b.size == b.limit {
			b.head = (b.head + 1) % b.limit
			continue
		}
		b.size++
	}

	return len(frames)
}

// Snapshot returns up to the newest maxFrames in chronological order.
func (b *RingBuffer) Snapshot(maxFrames int) []float32 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return nil
	}

	count := b.size
	if maxFrames > 0 && maxFrames < count {
		count = maxFrames
	}

	start := b.head
	if count < b.size {
		start = (b.head + (b.size - count)) % b.limit
	}

	frames := make([]float32, count)
	for index := 0; index < count; index++ {
		frames[index] = b.data[(start+index)%b.limit]
	}

	return frames
}

// Len returns the current number of retained frames.
func (b *RingBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}

// Cap returns the maximum number of retained frames.
func (b *RingBuffer) Cap() int {
	return b.limit
}
