package pagestream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/testutil/ssetest"
)

func TestSignalStreamPatchSendsOnePatchSignalsEventPerCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	stream := NewSignalStream(rec, req)
	if err := stream.Patch(SignalPatch{"status": "loading"}); err != nil {
		t.Fatalf("patch loading: %v", err)
	}
	if err := stream.Patch(SignalPatch{"status": "ready"}); err != nil {
		t.Fatalf("patch ready: %v", err)
	}

	patches := ssetest.PatchSignals(t, rec.Body.String())
	if len(patches) != 2 || patches[0]["status"] != "loading" || patches[1]["status"] != "ready" {
		t.Fatalf("stream patches = %#v", patches)
	}
}

func TestSignalStreamForwardRelaysBrokerPatchesAndCleansUp(t *testing.T) {
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)
		stream := NewSignalStream(rec, req)
		if err := stream.Forward(ctx, broker, "client:page"); err != nil {
			t.Errorf("forward: %v", err)
		}
	}()

	waitFor(t, time.Second, func() bool {
		return broker.SubscriberCount("client:page") == 1
	})
	broker.Publish("client:page", SignalPatch{"status": "broker"})
	waitFor(t, time.Second, func() bool {
		return len(ssetest.PatchSignals(t, rec.Body.String())) >= 1
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
	if len(patches) != 1 || patches[0]["status"] != "broker" {
		t.Fatalf("stream patches = %#v", patches)
	}
}

func TestSignalStreamForwardRequiresBrokerAndStreamID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	if err := NewSignalStream(rec, req).Forward(ctx, nil, "client:page"); err == nil {
		t.Fatal("Forward with nil broker returned nil error")
	}
	if err := NewSignalStream(rec, req).Forward(ctx, NewBroker(), ""); err == nil {
		t.Fatal("Forward with empty stream id returned nil error")
	}
}

func TestSignalStreamWaitStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)
		NewSignalStream(rec, req).Wait(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after cancellation")
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
