package module

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/runtimehost"
	"github.com/flidai/leapview/internal/servingstate"
)

type reaperTestRepo struct{}

func (reaperTestRepo) ActiveArtifact(context.Context, servingstate.WorkspaceID, servingstate.Environment) (servingstate.State, servingstate.Artifact, error) {
	return servingstate.State{}, servingstate.Artifact{}, servingstate.ErrNotFound
}
func (reaperTestRepo) ByID(context.Context, servingstate.ID) (servingstate.State, error) {
	return servingstate.State{}, servingstate.ErrNotFound
}
func (reaperTestRepo) ArtifactByServingState(context.Context, servingstate.ID) (servingstate.Artifact, error) {
	return servingstate.Artifact{}, servingstate.ErrNotFound
}
func (reaperTestRepo) RecordDuckLakeSnapshot(context.Context, servingstate.ID, int64) error {
	return nil
}

type reaperTestFactory struct{}

func (reaperTestFactory) Prepare(context.Context, runtimehost.RuntimeInput) (runtimehost.Runtime, error) {
	return reaperTestRuntime{}, nil
}

type reaperTestRuntime struct{}

func (reaperTestRuntime) Close() error { return nil }

func TestCandidateReaperRunsAndCloseJoins(t *testing.T) {
	var ticks atomic.Int32
	m, err := Build(context.Background(), Config{
		States: reaperTestRepo{}, Factory: reaperTestFactory{}, CandidateReapInterval: time.Millisecond,
		OnCandidateReap: func(int) { ticks.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for ticks.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if ticks.Load() == 0 {
		t.Fatal("candidate reaper did not run")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	count := ticks.Load()
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if ticks.Load() != count {
		t.Fatalf("candidate reaper continued after close: before=%d after=%d", count, ticks.Load())
	}
}

func TestModuleCloseIsSafeForConcurrentCallers(t *testing.T) {
	m, err := Build(context.Background(), Config{States: reaperTestRepo{}, Factory: reaperTestFactory{}, CandidateReapInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	const callers = 8
	errs := make(chan error, callers)
	for range callers {
		go func() { errs <- m.Close() }()
	}
	for range callers {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent close: %v", err)
		}
	}
}
