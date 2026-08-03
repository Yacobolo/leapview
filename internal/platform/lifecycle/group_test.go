package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
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
	entered := make(chan struct{})
	release := make(chan struct{})
	var firstStops atomic.Int32
	var secondStarts atomic.Int32
	group := New(
		Component{
			Start: func(context.Context) error {
				close(entered)
				<-release
				return nil
			},
			Stop: func(context.Context) error {
				firstStops.Add(1)
				return nil
			},
		},
		Component{Start: func(context.Context) error {
			secondStarts.Add(1)
			return nil
		}},
	)
	startResult := make(chan error, 1)
	go func() { startResult <- group.Start(context.Background()) }()
	<-entered

	stopContext, cancelStop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err := group.Stop(stopContext)
	cancelStop()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() during blocked startup error = %v, want deadline exceeded", err)
	}
	close(release)
	if err := <-startResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() after concurrent stop = %v, want canceled", err)
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := secondStarts.Load(); got != 0 {
		t.Fatalf("later component starts = %d, want 0", got)
	}
	if got := firstStops.Load(); got != 1 {
		t.Fatalf("started component stops = %d, want 1", got)
	}
}

func TestGroupStartWaitsForConcurrentStopBeforeRestarting(t *testing.T) {
	stopEntered := make(chan struct{})
	releaseStop := make(chan struct{})
	var starts atomic.Int32
	var stops atomic.Int32
	group := New(Component{
		Start: func(context.Context) error {
			starts.Add(1)
			return nil
		},
		Stop: func(context.Context) error {
			if stops.Add(1) == 1 {
				close(stopEntered)
				<-releaseStop
			}
			return nil
		},
	})
	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopResult := make(chan error, 1)
	go func() { stopResult <- group.Stop(context.Background()) }()
	<-stopEntered
	startResult := make(chan error, 1)
	go func() { startResult <- group.Start(context.Background()) }()
	select {
	case err := <-startResult:
		t.Fatalf("Start() returned before concurrent Stop(): %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseStop)
	if err := <-stopResult; err != nil {
		t.Fatal(err)
	}
	if err := <-startResult; err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 2 {
		t.Fatalf("component starts = %d, want restart after stop", got)
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
