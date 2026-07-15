package pagestream

import (
	"reflect"
	"sync"
	"time"
)

// SignalPatch is a Datastar signal patch. pagestream intentionally streams
// signal patches only; it does not transport element morphs or scripts.
type SignalPatch map[string]any

// Broker fans envelopes out to every subscriber of a stream. Delivery
// semantics are explicit on Envelope; the broker never inspects application
// signal shapes to discover generations or boundaries.
type Broker struct {
	mu      sync.Mutex
	clients map[string]map[*brokerSubscription]struct{}
	trace   *TraceStore
}

type BrokerOption func(*Broker)

func WithTraceStore(store *TraceStore) BrokerOption {
	return func(broker *Broker) { broker.trace = store }
}

type brokerSubscription struct {
	mu            sync.Mutex
	pending       []pendingEnvelope
	generation    uint64
	hasGeneration bool
	closed        bool
	out           chan SignalPatch
	wake          chan struct{}
	done          chan struct{}
	once          sync.Once
}

type pendingEnvelope struct {
	envelope Envelope
}

const maxPendingEnvelopes = 64

func NewBroker(options ...BrokerOption) *Broker {
	broker := &Broker{clients: map[string]map[*brokerSubscription]struct{}{}}
	for _, option := range options {
		if option != nil {
			option(broker)
		}
	}
	return broker
}

func (b *Broker) SetTraceStore(store *TraceStore) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.trace = store
	b.mu.Unlock()
}

func (b *Broker) TraceStore() *TraceStore {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.trace
}

func (b *Broker) Subscribe(streamID string) (<-chan SignalPatch, func()) {
	subscription := &brokerSubscription{
		out:  make(chan SignalPatch, 1),
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}

	b.mu.Lock()
	if b.clients[streamID] == nil {
		b.clients[streamID] = map[*brokerSubscription]struct{}{}
	}
	b.clients[streamID][subscription] = struct{}{}
	b.mu.Unlock()

	go subscription.forward()
	return subscription.out, func() {
		subscription.once.Do(func() {
			b.mu.Lock()
			delete(b.clients[streamID], subscription)
			if len(b.clients[streamID]) == 0 {
				delete(b.clients, streamID)
			}
			b.mu.Unlock()
			subscription.close()
		})
	}
}

// Publish uses the safe lossless default: each patch is an observable
// boundary. High-volume producers should use PublishEnvelope and explicitly
// declare their coalescing policy.
func (b *Broker) Publish(streamID string, patch SignalPatch) {
	b.PublishEnvelope(streamID, Envelope{
		Signals:  patch,
		Delivery: DeliveryMetadata{Boundary: true},
	})
}

func (b *Broker) PublishEnvelope(streamID string, envelope Envelope) {
	if b == nil || len(envelope.Signals) == 0 || streamID == "" {
		return
	}
	b.mu.Lock()
	store := b.trace
	subscriptions := make([]*brokerSubscription, 0, len(b.clients[streamID]))
	for subscription := range b.clients[streamID] {
		subscriptions = append(subscriptions, subscription)
	}
	b.mu.Unlock()

	if store != nil {
		event := store.Record(TraceRecord{
			StreamID:      streamID,
			Stage:         TraceStagePublished,
			Signals:       envelope.Signals,
			Generation:    envelope.Delivery.Generation,
			Origin:        envelope.Trace.Origin,
			CorrelationID: envelope.Trace.CorrelationID,
		})
		envelope.trace = &traceSpan{
			store: store, streamID: streamID, sequence: event.Sequence,
			publishedAt: time.Now().UnixNano(), coalesced: 1,
		}
	}
	for _, subscription := range subscriptions {
		subscription.enqueue(envelope)
	}
}

func (b *Broker) SubscriberCount(streamID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients[streamID])
}

func (s *brokerSubscription) enqueue(envelope Envelope) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	generation := envelope.Delivery.Generation
	if generation > 0 && s.hasGeneration && generation < s.generation {
		recordEnvelopeTrace(envelope, TraceStageDropped, "stale_generation", 0)
		s.mu.Unlock()
		return
	}
	if generation > 0 && (!s.hasGeneration || generation > s.generation) {
		s.generation = generation
		s.hasGeneration = true
		kept := s.pending[:0]
		for _, pending := range s.pending {
			if pending.envelope.Delivery.Generation >= generation {
				kept = append(kept, pending)
			} else {
				recordEnvelopeTrace(pending.envelope, TraceStageDropped, "superseded_generation", 0)
			}
		}
		s.pending = kept
		select {
		case <-s.out:
		default:
		}
	}

	next := pendingEnvelope{envelope: cloneEnvelope(envelope)}
	switch {
	case len(s.pending) == 0:
		s.pending = append(s.pending, next)
	case shouldCoalesce(s.pending[len(s.pending)-1].envelope, next.envelope):
		last := len(s.pending) - 1
		s.pending[last].envelope = coalesceEnvelopes(s.pending[last].envelope, next.envelope)
	case len(s.pending) < maxPendingEnvelopes:
		s.pending = append(s.pending, next)
	default:
		recordEnvelopeTrace(s.pending[0].envelope, TraceStageDropped, "mailbox_capacity", 0)
		copy(s.pending, s.pending[1:])
		s.pending[len(s.pending)-1] = next
	}
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func shouldCoalesce(current, next Envelope) bool {
	if current.Delivery.Boundary || next.Delivery.Boundary {
		return false
	}
	if current.Delivery.Generation != next.Delivery.Generation {
		return false
	}
	return current.Delivery.CoalesceGroup != "" && current.Delivery.CoalesceGroup == next.Delivery.CoalesceGroup
}

func coalesceEnvelopes(current, next Envelope) Envelope {
	mergeRoots := make(map[string]struct{}, len(current.Delivery.MergeRoots)+len(next.Delivery.MergeRoots))
	for _, root := range current.Delivery.MergeRoots {
		mergeRoots[root] = struct{}{}
	}
	for _, root := range next.Delivery.MergeRoots {
		mergeRoots[root] = struct{}{}
	}
	next.Signals = coalesceSignalPatches(current.Signals, next.Signals, mergeRoots)
	if next.trace != nil {
		count := 1
		if current.trace != nil && current.trace.coalesced > 0 {
			count += current.trace.coalesced
		}
		next.trace.coalesced = count
		recordEnvelopeTrace(next, TraceStageCoalesced, "same_group", 0)
	}
	return next
}

func cloneEnvelope(envelope Envelope) Envelope {
	envelope.Signals = coalesceSignalPatches(nil, envelope.Signals, nil)
	envelope.Delivery.MergeRoots = append([]string(nil), envelope.Delivery.MergeRoots...)
	return envelope
}

func recordEnvelopeTrace(envelope Envelope, stage TraceStage, outcome string, queueMilliseconds float64) {
	if envelope.trace == nil || envelope.trace.store == nil {
		return
	}
	envelope.trace.store.Record(TraceRecord{
		StreamID:          envelope.trace.streamID,
		Stage:             stage,
		Signals:           envelope.Signals,
		Sequence:          envelope.trace.sequence,
		Generation:        envelope.Delivery.Generation,
		Origin:            envelope.Trace.Origin,
		CorrelationID:     envelope.Trace.CorrelationID,
		QueueMilliseconds: queueMilliseconds,
		Coalesced:         envelope.trace.coalesced,
		Outcome:           outcome,
	})
}

func (s *brokerSubscription) forward() {
	defer close(s.out)
	for {
		s.mu.Lock()
		pending := len(s.pending) > 0
		sent := false
		if pending {
			envelope := s.pending[0].envelope
			select {
			case s.out <- envelope.Signals:
				s.pending = s.pending[1:]
				queueMilliseconds := float64(0)
				if envelope.trace != nil && envelope.trace.publishedAt > 0 {
					queueMilliseconds = float64(time.Now().UnixNano()-envelope.trace.publishedAt) / float64(time.Millisecond)
				}
				recordEnvelopeTrace(envelope, TraceStageDelivered, "", queueMilliseconds)
				sent = true
			default:
			}
		}
		s.mu.Unlock()
		if sent {
			continue
		}
		if !pending {
			select {
			case <-s.done:
				return
			case <-s.wake:
			}
			continue
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-s.done:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-s.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

func (s *brokerSubscription) close() {
	s.mu.Lock()
	s.closed = true
	s.pending = nil
	s.mu.Unlock()
	close(s.done)
}

func coalesceSignalPatches(current, next SignalPatch, mergeRoots map[string]struct{}) SignalPatch {
	result := make(SignalPatch, len(current)+len(next))
	for key, value := range current {
		result[key] = value
	}
	for key, value := range next {
		if _, merge := mergeRoots[key]; merge {
			if combined, ok := mergeStringMaps(result[key], value); ok {
				result[key] = combined
				continue
			}
		}
		result[key] = value
	}
	return result
}

// mergeStringMaps preserves concrete signal contract map types.
func mergeStringMaps(current, next any) (any, bool) {
	nextValue := reflect.ValueOf(next)
	if !nextValue.IsValid() || nextValue.Kind() != reflect.Map || nextValue.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	currentValue := reflect.ValueOf(current)
	if current == nil {
		currentValue = reflect.MakeMap(nextValue.Type())
	}
	if !currentValue.IsValid() || currentValue.Kind() != reflect.Map || currentValue.Type() != nextValue.Type() {
		return nil, false
	}
	merged := reflect.MakeMapWithSize(nextValue.Type(), currentValue.Len()+nextValue.Len())
	iterator := currentValue.MapRange()
	for iterator.Next() {
		merged.SetMapIndex(iterator.Key(), iterator.Value())
	}
	iterator = nextValue.MapRange()
	for iterator.Next() {
		merged.SetMapIndex(iterator.Key(), iterator.Value())
	}
	return merged.Interface(), true
}
