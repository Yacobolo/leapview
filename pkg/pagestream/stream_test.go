package pagestream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/testutil/ssetest"
)

func TestServeStreamSendsInitialAndBrokerPatchesAndCleansUp(t *testing.T) {
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)
		ServeStream(rec, req, StreamSpec{
			Broker:         broker,
			StreamID:       "client:page",
			InitialPatches: []Patch{{"status": "initial"}},
		})
	}()

	waitFor(t, time.Second, func() bool {
		return broker.SubscriberCount("client:page") == 1
	})
	broker.Publish("client:page", Patch{"status": "broker"})
	waitFor(t, time.Second, func() bool {
		return len(ssetest.PatchSignals(t, rec.Body.String())) >= 2
	})

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after cancellation")
	}
	if got := broker.SubscriberCount("client:page"); got != 0 {
		t.Fatalf("subscriber count after cancellation = %d, want 0", got)
	}

	patches := ssetest.PatchSignals(t, rec.Body.String())
	if patches[0]["status"] != "initial" || patches[1]["status"] != "broker" {
		t.Fatalf("stream patches = %#v", patches)
	}
}

func TestServeStreamSendsSnapshotAndTickerPatches(t *testing.T) {
	var count atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)
		ServeStream(rec, req, StreamSpec{
			Snapshot: func(context.Context) []Patch {
				next := count.Add(1)
				return []Patch{{"tick": next}}
			},
			TickerInterval: 5 * time.Millisecond,
		})
	}()

	waitFor(t, time.Second, func() bool {
		return count.Load() >= 2
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after cancellation")
	}

	patches := ssetest.PatchSignals(t, rec.Body.String())
	if len(patches) < 2 {
		t.Fatalf("snapshot stream patches = %#v, want at least two", patches)
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
