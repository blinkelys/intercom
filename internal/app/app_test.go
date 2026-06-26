package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"procom/internal/config"
)

func TestAppRunStartsAndStopsComponentsInOrder(t *testing.T) {
	t.Parallel()

	tracker := &callTracker{}
	application, err := New(
		WithShutdownTimeout(time.Second),
		WithComponentFactories(
			stubFactory("one", tracker, nil, nil),
			stubFactory("two", tracker, nil, nil),
		),
	)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- application.Run(ctx)
	}()

	cancel()

	if err := <-done; err != nil {
		t.Fatalf("run app: %v", err)
	}

	want := []string{"start:one", "start:two", "stop:two", "stop:one"}
	if !reflect.DeepEqual(tracker.calls, want) {
		t.Fatalf("calls = %v, want %v", tracker.calls, want)
	}
}

func TestAppRunRollsBackStartedComponentsOnFailure(t *testing.T) {
	t.Parallel()

	tracker := &callTracker{}
	application, err := New(
		WithConfigSource(config.StaticSource{Config: config.Default()}),
		WithComponentFactories(
			stubFactory("one", tracker, nil, nil),
			stubFactory("two", tracker, errors.New("boom"), nil),
		),
	)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	err = application.Run(context.Background())
	if err == nil {
		t.Fatal("expected startup failure")
	}

	want := []string{"start:one", "start:two", "stop:one"}
	if !reflect.DeepEqual(tracker.calls, want) {
		t.Fatalf("calls = %v, want %v", tracker.calls, want)
	}
}

type callTracker struct {
	calls []string
}

type stubComponent struct {
	name     string
	tracker  *callTracker
	startErr error
	stopErr  error
}

func (c *stubComponent) Name() string {
	return c.name
}

func (c *stubComponent) Start(context.Context) error {
	c.tracker.calls = append(c.tracker.calls, "start:"+c.name)
	return c.startErr
}

func (c *stubComponent) Stop(context.Context) error {
	c.tracker.calls = append(c.tracker.calls, "stop:"+c.name)
	return c.stopErr
}

func stubFactory(name string, tracker *callTracker, startErr error, stopErr error) ComponentFactory {
	return func(Dependencies) (Component, error) {
		return &stubComponent{
			name:     name,
			tracker:  tracker,
			startErr: startErr,
			stopErr:  stopErr,
		}, nil
	}
}
