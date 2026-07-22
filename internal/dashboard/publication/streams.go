package publication

import (
	"context"
	"sync"

	"github.com/Yacobolo/leapview/internal/dashboard"
	"github.com/Yacobolo/leapview/internal/dashboard/command"
)

type StreamVersion struct {
	PublicID       string
	ServingStateID string
}

type StreamRegistry interface {
	Register(context.Context, string, string, StreamVersion, ...dashboard.Filters) (context.Context, func(), error)
	PrepareCommand(context.Context, string, string, StreamVersion, func(dashboard.Filters) (command.PreparedRefresh, error)) (command.PreparedRefresh, uint64, error)
	Active(string, string, StreamVersion) bool
	Reconcile(context.Context, map[string]StreamVersion)
	ClosePublication(string)
}

type memoryStreamRegistry struct {
	mu      sync.Mutex
	streams map[string]map[string]*memoryStream
}

type memoryStream struct {
	cancel  context.CancelFunc
	version StreamVersion
}

func NewMemoryStreamRegistry() StreamRegistry {
	return &memoryStreamRegistry{streams: map[string]map[string]*memoryStream{}}
}

func (r *memoryStreamRegistry) Register(parent context.Context, publicationID, streamID string, version StreamVersion, _ ...dashboard.Filters) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(parent)
	r.mu.Lock()
	if r.streams[publicationID] == nil {
		r.streams[publicationID] = map[string]*memoryStream{}
	}
	if previous := r.streams[publicationID][streamID]; previous != nil {
		previous.cancel()
	}
	registration := &memoryStream{cancel: cancel, version: version}
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
	}, nil
}

func (r *memoryStreamRegistry) PrepareCommand(context.Context, string, string, StreamVersion, func(dashboard.Filters) (command.PreparedRefresh, error)) (command.PreparedRefresh, uint64, error) {
	return command.PreparedRefresh{}, 0, ErrStreamStateUnavailable
}

func (r *memoryStreamRegistry) Active(publicationID, streamID string, version StreamVersion) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	stream := r.streams[publicationID][streamID]
	return stream != nil && stream.version == version
}

func (r *memoryStreamRegistry) Reconcile(_ context.Context, active map[string]StreamVersion) {
	r.mu.Lock()
	stale := []context.CancelFunc{}
	for publicationID, streams := range r.streams {
		current, ok := active[publicationID]
		for streamID, stream := range streams {
			if ok && stream.version == current {
				continue
			}
			stale = append(stale, stream.cancel)
			delete(streams, streamID)
		}
		if len(streams) == 0 {
			delete(r.streams, publicationID)
		}
	}
	r.mu.Unlock()
	for _, cancel := range stale {
		cancel()
	}
}

func (r *memoryStreamRegistry) ClosePublication(publicationID string) {
	r.mu.Lock()
	streams := r.streams[publicationID]
	delete(r.streams, publicationID)
	r.mu.Unlock()
	for _, stream := range streams {
		stream.cancel()
	}
}
