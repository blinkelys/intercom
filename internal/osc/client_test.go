package osc

import (
	"bytes"
	"testing"
)

func TestEncodeProducesPaddedOSCPayload(t *testing.T) {
	t.Parallel()

	payload, err := Encode(Message{Address: "/go", Arguments: []string{"now"}})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if len(payload)%4 != 0 {
		t.Fatalf("payload length mod 4 = %d, want 0", len(payload)%4)
	}
	if !bytes.Contains(payload, []byte("/go")) {
		t.Fatalf("payload = %v, want address bytes", payload)
	}
	if !bytes.Contains(payload, []byte(",s")) {
		t.Fatalf("payload = %v, want type tags", payload)
	}
}
