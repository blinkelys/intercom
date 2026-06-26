package audio

import (
	"reflect"
	"testing"
)

func TestRingBufferSnapshotReturnsNewestFrames(t *testing.T) {
	t.Parallel()

	buffer, err := NewRingBuffer(4)
	if err != nil {
		t.Fatalf("new ring buffer: %v", err)
	}

	buffer.Write([]float32{1, 2, 3})
	buffer.Write([]float32{4, 5})

	got := buffer.Snapshot(0)
	want := []float32{2, 3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot = %v, want %v", got, want)
	}
}
