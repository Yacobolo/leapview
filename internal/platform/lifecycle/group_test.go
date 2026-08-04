package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestGroupStartsInOrderAndStopsInReverse(t *testing.T) {
	events := []string{}
	component := func(name string) Component {
		return Component{
			Start: func(context.Context) error { events = append(events, "start:"+name); return nil },
			Stop:  func(context.Context) error { events = append(events, "stop:"+name); return nil },
		}
	}
	group := New(component("dependency"), component("worker"))
	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:dependency", "start:worker", "stop:worker", "stop:dependency"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestGroupRollsBackStartedComponents(t *testing.T) {
	events := []string{}
	wantErr := errors.New("boom")
	group := New(
		Component{
			Start: func(context.Context) error { events = append(events, "start:one"); return nil },
			Stop:  func(context.Context) error { events = append(events, "stop:one"); return nil },
		},
		Component{Start: func(context.Context) error { events = append(events, "start:two"); return wantErr }},
	)
	if err := group.Start(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Start() error = %v, want %v", err, wantErr)
	}
	want := []string{"start:one", "start:two", "stop:one"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestGroupStopDuringStartupPreventsLaterComponents(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	stopCtxErr := make(chan error, 1)
	var mu sync.Mutex
	events := []string{}
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}
	group := New(
		Component{Start: func(context.Context) error { record("start:one"); return nil }, Stop: func(context.Context) error { record("stop:one"); return nil }},
		Component{Start: func(context.Context) error { record("start:two"); close(started); <-release; return nil }, Stop: func(ctx context.Context) error { record("stop:two"); stopCtxErr <- ctx.Err(); return nil }},
		Component{Start: func(context.Context) error { record("start:three"); return nil }, Stop: func(context.Context) error { record("stop:three"); return nil }},
	)
	startDone := make(chan error, 1)
	go func() { startDone <- group.Start(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("component two did not start")
	}
	stopDone := make(chan error, 1)
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelStop()
	go func() { stopDone <- group.Stop(stopCtx) }()
	select {
	case err := <-stopDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not honor its context")
	}
	close(release)
	if err := <-startDone; err != nil {
		t.Fatal(err)
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-stopCtxErr; err != nil {
		t.Fatalf("cleanup stop context = %v, want nil", err)
	}
	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()
	want := []string{"start:one", "start:two", "stop:two", "stop:one"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestGroupStopReturnsTerminalErrorAndRepeatsIt(t *testing.T) {
	stopErr := errors.New("stop failed")
	group := New(Component{
		Start: func(context.Context) error { return nil },
		Stop:  func(context.Context) error { return stopErr },
	})
	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := group.Stop(context.Background()); !errors.Is(err, stopErr) {
		t.Fatalf("Stop() error = %v, want %v", err, stopErr)
	}
	if err := group.Stop(context.Background()); !errors.Is(err, stopErr) {
		t.Fatalf("repeated Stop() error = %v, want %v", err, stopErr)
	}
}

func TestGroupStopDuringStartupReturnsTerminalError(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	stopErr := errors.New("stop failed")
	group := New(Component{
		Start: func(context.Context) error { close(started); <-release; return nil },
		Stop:  func(context.Context) error { return stopErr },
	})
	startDone := make(chan error, 1)
	go func() { startDone <- group.Start(context.Background()) }()
	<-started
	stopDone := make(chan error, 1)
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	go func() { stopDone <- group.Stop(stopCtx) }()
	if err := <-stopDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("initial Stop() error = %v, want deadline exceeded", err)
	}
	close(release)
	if err := <-startDone; !errors.Is(err, stopErr) {
		t.Fatalf("Start() error = %v, want %v", err, stopErr)
	}
	if err := group.Stop(context.Background()); !errors.Is(err, stopErr) {
		t.Fatalf("repeated Stop() error = %v, want %v", err, stopErr)
	}
}

func TestGroupCanceledStartUsesBoundedCleanupContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	stopChecked := make(chan error, 1)
	group := New(Component{
		Start: func(ctx context.Context) error { close(started); <-ctx.Done(); return nil },
		Stop: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				stopChecked <- errors.New("stop context has no deadline")
				return nil
			}
			stopChecked <- ctx.Err()
			return nil
		},
	})
	startDone := make(chan error, 1)
	go func() { startDone <- group.Start(ctx) }()
	<-started
	cancel()
	if err := <-startDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v, want canceled", err)
	}
	if err := <-stopChecked; err != nil {
		t.Fatalf("stop context error = %v, want live bounded context", err)
	}
}
