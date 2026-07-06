package pagestream

import "sync"

// SignalPatch is a Datastar signal patch. pagestream intentionally streams
// signal patches only; it does not transport element morphs or scripts.
type SignalPatch map[string]any

type Broker struct {
	mu      sync.Mutex
	clients map[string]map[chan SignalPatch]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: map[string]map[chan SignalPatch]struct{}{}}
}

func (b *Broker) Subscribe(streamID string) (<-chan SignalPatch, func()) {
	ch := make(chan SignalPatch, 8)

	b.mu.Lock()
	if b.clients[streamID] == nil {
		b.clients[streamID] = map[chan SignalPatch]struct{}{}
	}
	b.clients[streamID][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.clients[streamID], ch)
		if len(b.clients[streamID]) == 0 {
			delete(b.clients, streamID)
		}
		close(ch)
	}
}

func (b *Broker) Publish(streamID string, patch SignalPatch) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.clients[streamID] {
		select {
		case ch <- patch:
		default:
		}
	}
}

func (b *Broker) SubscriberCount(streamID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients[streamID])
}
