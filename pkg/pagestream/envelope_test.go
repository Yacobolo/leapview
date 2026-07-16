package pagestream

import (
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestBrokerUsesExplicitGenerationMetadata(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	broker := NewBroker()
	updates, unsubscribe := broker.Subscribe("dashboard:page")
	defer unsubscribe()

	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"payload": "generation-one"},
		Delivery: DeliveryMetadata{Generation: 1, Boundary: true},
	})
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for generation one")
	}

	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"payload": "generation-two"},
		Delivery: DeliveryMetadata{Generation: 2, Boundary: true},
	})
	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"payload": "stale-without-generation-in-payload"},
		Delivery: DeliveryMetadata{Generation: 1, Boundary: true},
	})

	select {
	case patch := <-updates:
		if patch["payload"] != "generation-two" {
			t.Fatalf("patch = %#v, want generation two", patch)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for generation two")
	}
	select {
	case patch := <-updates:
		t.Fatalf("received stale patch %#v", patch)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestBrokerUsesExplicitBoundariesAndMergeRoots(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	broker := NewBroker()
	updates, unsubscribe := broker.Subscribe("dashboard:page")
	defer unsubscribe()

	metadata := DeliveryMetadata{
		Generation:    7,
		CoalesceGroup: "dashboard-results",
		MergeRoots:    []string{"visuals"},
	}
	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"status": "0/2"},
		Delivery: DeliveryMetadata{Generation: 7, Boundary: true},
	})
	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"visuals": map[string]any{"one": 1}},
		Delivery: metadata,
	})
	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"visuals": map[string]any{"two": 2}},
		Delivery: metadata,
	})
	broker.PublishEnvelope("dashboard:page", Envelope{
		Signals:  SignalPatch{"status": "1/2"},
		Delivery: DeliveryMetadata{Generation: 7, Boundary: true},
	})

	want := []SignalPatch{
		{"status": "0/2"},
		{"visuals": map[string]any{"one": 1, "two": 2}},
		{"status": "1/2"},
	}
	for index := range want {
		select {
		case patch := <-updates:
			if !reflect.DeepEqual(patch, want[index]) {
				t.Fatalf("patch %d = %#v, want %#v", index, patch, want[index])
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for patch %d", index)
		}
	}
}
