package app

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
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
