package stream

import "sync"

type Patch = map[string]any

type Broker struct {
	mu      sync.Mutex
	clients map[string]map[chan Patch]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: map[string]map[chan Patch]struct{}{}}
}

func (b *Broker) Subscribe(clientID string) (<-chan Patch, func()) {
	ch := make(chan Patch, 8)

	b.mu.Lock()
	if b.clients[clientID] == nil {
		b.clients[clientID] = map[chan Patch]struct{}{}
	}
	b.clients[clientID][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.clients[clientID], ch)
		if len(b.clients[clientID]) == 0 {
			delete(b.clients, clientID)
		}
		close(ch)
	}
}

func (b *Broker) Publish(clientID string, patch Patch) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.clients[clientID] {
		select {
		case ch <- patch:
		default:
		}
	}
}

func (b *Broker) SubscriberCount(clientID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients[clientID])
}
