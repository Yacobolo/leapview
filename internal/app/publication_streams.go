package app

import (
	"context"
	"sync"
)

type publicationStreamRegistry struct {
	mu      sync.Mutex
	streams map[string]map[string]*publicationStream
}

type publicationStream struct {
	cancel         context.CancelFunc
	servingStateID string
}

func newPublicationStreamRegistry() *publicationStreamRegistry {
	return &publicationStreamRegistry{streams: map[string]map[string]*publicationStream{}}
}

func (r *publicationStreamRegistry) Register(parent context.Context, publicationID, streamID, servingStateID string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	r.mu.Lock()
	if r.streams[publicationID] == nil {
		r.streams[publicationID] = map[string]*publicationStream{}
	}
	if previous := r.streams[publicationID][streamID]; previous != nil {
		previous.cancel()
	}
	registration := &publicationStream{cancel: cancel, servingStateID: servingStateID}
	r.streams[publicationID][streamID] = registration
	r.mu.Unlock()
	return ctx, func() {
		r.mu.Lock()
		if current := r.streams[publicationID][streamID]; current == registration {
			delete(r.streams[publicationID], streamID)
			if len(r.streams[publicationID]) == 0 {
				delete(r.streams, publicationID)
			}
		}
		r.mu.Unlock()
		cancel()
	}
}

func (r *publicationStreamRegistry) Active(publicationID, streamID, servingStateID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	stream := r.streams[publicationID][streamID]
	return stream != nil && stream.servingStateID == servingStateID
}

func (r *publicationStreamRegistry) ClosePublication(publicationID string) {
	r.mu.Lock()
	streams := r.streams[publicationID]
	delete(r.streams, publicationID)
	r.mu.Unlock()
	for _, cancel := range streams {
		cancel.cancel()
	}
}
