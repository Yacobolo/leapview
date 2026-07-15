package pagestream

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestTraceStoreRecordsPublishedAndDeliveredSanitizedEnvelopes(t *testing.T) {
	var logs bytes.Buffer
	store := NewTraceStore(TraceOptions{
		CapacityPerStream: 16,
		MaxStreams:        2,
		Logger:            slog.New(slog.NewJSONHandler(&logs, nil)),
		IncludePayloads:   true,
	})
	broker := NewBroker(WithTraceStore(store))
	updates, unsubscribe := broker.Subscribe("dashboard:page")
	defer unsubscribe()

	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals: SignalPatch{
			"status":   map[string]any{"loading": true},
			"password": "do-not-record",
		},
		Delivery: DeliveryMetadata{Generation: 3, Boundary: true},
		Trace:    TraceMetadata{Origin: "dashboard.refresh", CorrelationID: "refresh-3"},
	})
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for traced patch")
	}
	waitFor(t, time.Second, func() bool {
		return len(store.Events(TraceQuery{StreamID: "dashboard:page", Limit: 10})) >= 2
	})

	events := store.Events(TraceQuery{StreamID: "dashboard:page", Limit: 10})
	if len(events) != 2 || events[0].Stage != TraceStagePublished || events[1].Stage != TraceStageDelivered {
		t.Fatalf("events = %#v, want published and delivered", events)
	}
	delivered := events[1]
	if delivered.Sequence != 1 || delivered.Generation != 3 || delivered.CorrelationID != "refresh-3" || delivered.Origin != "dashboard.refresh" {
		t.Fatalf("delivered metadata = %#v", delivered)
	}
	if delivered.Bytes == 0 || delivered.Digest == "" || delivered.QueueMilliseconds < 0 {
		t.Fatalf("delivered diagnostics = %#v", delivered)
	}
	if got := delivered.Payload["password"]; got != "[REDACTED]" {
		t.Fatalf("sanitized password = %#v", got)
	}
	if strings.Contains(logs.String(), "do-not-record") || !strings.Contains(logs.String(), `"stage":"delivered"`) {
		t.Fatalf("trace logs = %s", logs.String())
	}
}

func TestTraceStoreIsBoundedAndSupportsIncrementalQueries(t *testing.T) {
	store := NewTraceStore(TraceOptions{CapacityPerStream: 2, MaxStreams: 1, IncludePayloads: true})
	store.Record(TraceRecord{StreamID: "one", Stage: TraceStagePublished, Signals: SignalPatch{"value": 1}})
	first := store.Events(TraceQuery{StreamID: "one", Limit: 10})
	store.Record(TraceRecord{StreamID: "one", Stage: TraceStagePublished, Signals: SignalPatch{"value": 2}})
	store.Record(TraceRecord{StreamID: "one", Stage: TraceStagePublished, Signals: SignalPatch{"value": 3}})

	events := store.Events(TraceQuery{StreamID: "one", After: first[0].ID, Limit: 10})
	if len(events) != 2 || events[0].Payload["value"] != float64(2) || events[1].Payload["value"] != float64(3) {
		t.Fatalf("bounded incremental events = %#v", events)
	}

	store.Record(TraceRecord{StreamID: "two", Stage: TraceStagePublished, Signals: SignalPatch{"value": 4}})
	if events := store.Events(TraceQuery{StreamID: "one", Limit: 10}); len(events) != 0 {
		t.Fatalf("evicted stream events = %#v", events)
	}
}

func TestTraceStoreIncrementalLimitReturnsOldestUnreadEventsFirst(t *testing.T) {
	store := NewTraceStore(TraceOptions{CapacityPerStream: 8, MaxStreams: 1, IncludePayloads: true})
	for value := 1; value <= 3; value++ {
		store.Record(TraceRecord{StreamID: "one", Stage: TraceStagePublished, Signals: SignalPatch{"value": value}})
	}
	first := store.Events(TraceQuery{StreamID: "one", Limit: 2})
	if len(first) != 2 || first[0].Payload["value"] != float64(1) || first[1].Payload["value"] != float64(2) {
		t.Fatalf("first page = %#v", first)
	}
	second := store.Events(TraceQuery{StreamID: "one", After: first[1].ID, Limit: 2})
	if len(second) != 1 || second[0].Payload["value"] != float64(3) {
		t.Fatalf("second page = %#v", second)
	}
}
