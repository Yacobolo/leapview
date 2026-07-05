package pagestream

import (
	"testing"
	"time"
)

func TestBrokerPublishSubscribeAndUnsubscribe(t *testing.T) {
	broker := NewBroker()
	updates, unsubscribe := broker.Subscribe("client:page")

	if got := broker.SubscriberCount("client:page"); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}

	broker.Publish("client:page", SignalPatch{"status": "ready"})
	select {
	case patch := <-updates:
		if patch["status"] != "ready" {
			t.Fatalf("patch = %#v", patch)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker patch")
	}

	unsubscribe()
	if got := broker.SubscriberCount("client:page"); got != 0 {
		t.Fatalf("subscriber count after unsubscribe = %d, want 0", got)
	}
}

func TestBrokerPublishDoesNotBlockWhenSubscriberChannelIsFull(t *testing.T) {
	broker := NewBroker()
	_, unsubscribe := broker.Subscribe("client:page")
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 32; i++ {
			broker.Publish("client:page", SignalPatch{"seq": i})
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on a full subscriber channel")
	}
}
