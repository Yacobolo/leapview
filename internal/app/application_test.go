package app

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	deploymentmodule "github.com/flidai/leapview/internal/deployment/module"
)

type recordedLifecycle struct {
	name     string
	events   *[]string
	startErr error
}

func (l recordedLifecycle) Start(context.Context) error {
	*l.events = append(*l.events, "start:"+l.name)
	return l.startErr
}

func (l recordedLifecycle) Stop(context.Context) error {
	*l.events = append(*l.events, "stop:"+l.name)
	return nil
}

func TestApplicationStopsStartedComponentsWhenStartupFails(t *testing.T) {
	events := []string{}
	application := newApplication(http.NotFoundHandler(), []Lifecycle{
		recordedLifecycle{name: "one", events: &events},
		recordedLifecycle{name: "two", events: &events, startErr: errors.New("boom")},
	}, func(context.Context) error { events = append(events, "cleanup"); return nil })
	if err := application.Start(context.Background()); err == nil {
		t.Fatal("Start() accepted a component startup failure")
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:one", "start:two", "stop:one", "cleanup"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

type fatalLifecycle struct {
	recordedLifecycle
	fatal chan error
}

func (l fatalLifecycle) Fatal() <-chan error { return l.fatal }

func TestApplicationForwardsCapabilityFatalErrors(t *testing.T) {
	events := []string{}
	fatal := make(chan error, 1)
	application := newApplication(http.NotFoundHandler(), []Lifecycle{fatalLifecycle{
		recordedLifecycle: recordedLifecycle{name: "analytics", events: &events}, fatal: fatal,
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := application.Start(ctx); err != nil {
		t.Fatal(err)
	}
	want := errors.New("analytical failure")
	fatal <- want
	select {
	case got := <-application.Fatal():
		if !errors.Is(got, want) {
			t.Fatalf("Fatal() = %v, want %v", got, want)
		}
	case <-ctx.Done():
		t.Fatal("fatal error was not forwarded")
	}
}

func TestApplicationShutdownIsReverseOrderedAndIdempotent(t *testing.T) {
	events := []string{}
	application := newApplication(http.NotFoundHandler(), []Lifecycle{
		recordedLifecycle{name: "one", events: &events},
		recordedLifecycle{name: "two", events: &events},
	},
		func(context.Context) error { events = append(events, "cleanup:one"); return nil },
		func(context.Context) error { events = append(events, "cleanup:two"); return nil },
	)
	if err := application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:one", "start:two", "stop:two", "stop:one", "cleanup:two", "cleanup:one"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestApplicationShutdownDuringStartupPreventsLaterComponents(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var firstStops atomic.Int32
	var secondStarts atomic.Int32
	var cleanupCalls atomic.Int32
	application := newApplication(http.NotFoundHandler(), []Lifecycle{
		blockingApplicationLifecycle{entered: entered, release: release, stops: &firstStops},
		countingApplicationLifecycle{starts: &secondStarts},
	}, func(context.Context) error {
		cleanupCalls.Add(1)
		return nil
	})
	startResult := make(chan error, 1)
	go func() { startResult <- application.Start(context.Background()) }()
	<-entered

	stopContext, cancelStop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err := application.Shutdown(stopContext)
	cancelStop()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() during blocked startup error = %v, want deadline exceeded", err)
	}
	close(release)
	if err := <-startResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() after concurrent shutdown = %v, want canceled", err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := secondStarts.Load(); got != 0 {
		t.Fatalf("later component starts = %d, want 0", got)
	}
	if got := firstStops.Load(); got != 1 {
		t.Fatalf("started component stops = %d, want 1", got)
	}
	if got := cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

type blockingApplicationLifecycle struct {
	entered chan<- struct{}
	release <-chan struct{}
	stops   *atomic.Int32
}

func (l blockingApplicationLifecycle) Start(context.Context) error {
	close(l.entered)
	<-l.release
	return nil
}

func (l blockingApplicationLifecycle) Stop(context.Context) error {
	l.stops.Add(1)
	return nil
}

type countingApplicationLifecycle struct {
	starts *atomic.Int32
}

func (l countingApplicationLifecycle) Start(context.Context) error {
	l.starts.Add(1)
	return nil
}

func (countingApplicationLifecycle) Stop(context.Context) error { return nil }

func TestAssembleRuntimeRejectsCapabilityBuildFailure(t *testing.T) {
	store := testStore(t)
	options := testStoreOptions(store, assemblyConfig{
		DefaultWorkspaceID: "test",
		DeploymentConfig: deploymentmodule.Config{
			Database: store.SQLDB(),
		},
	})

	_, err := assembleRuntimeChecked(context.Background(), fakeMetrics{}, options)
	if err == nil {
		t.Fatal("assembleRuntimeChecked accepted an incomplete deployment capability")
	}
}
