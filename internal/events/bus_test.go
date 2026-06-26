package events

import "testing"

func TestBusPublishesToAllSubscribers(t *testing.T) {
	t.Parallel()

	bus, err := NewBus(1)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}

	first, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	defer first.Close()

	second, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe second: %v", err)
	}
	defer second.Close()

	event := Event{Type: "transcript.finalized", Payload: "hello"}
	if ok := bus.Publish(event); !ok {
		t.Fatal("expected publish to succeed")
	}

	if received := <-first.Events(); received.Type != event.Type {
		t.Fatalf("first subscriber got %q, want %q", received.Type, event.Type)
	}

	if received := <-second.Events(); received.Type != event.Type {
		t.Fatalf("second subscriber got %q, want %q", received.Type, event.Type)
	}
}

func TestBusPublishReportsBackpressure(t *testing.T) {
	t.Parallel()

	bus, err := NewBus(1)
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}

	subscription, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	if ok := bus.Publish(Event{Type: "one"}); !ok {
		t.Fatal("first publish should succeed")
	}

	if ok := bus.Publish(Event{Type: "two"}); ok {
		t.Fatal("second publish should report backpressure")
	}
}
