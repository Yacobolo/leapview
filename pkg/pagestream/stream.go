package pagestream

import (
	"context"
	"errors"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
)

var errMissingForwardTarget = errors.New("pagestream: broker and streamID are required")

// SignalStream is one long-lived Datastar SSE response that emits signal
// patches.
type SignalStream struct {
	sse      *datastar.ServerSentEventGenerator
	trace    *TraceStore
	streamID string
	origin   string
}

type SignalStreamOption func(*SignalStream)

func WithStreamTrace(store *TraceStore, streamID, origin string) SignalStreamOption {
	return func(stream *SignalStream) {
		stream.trace = store
		stream.streamID = streamID
		stream.origin = origin
	}
}

// NewSignalStream opens a Datastar SSE signal stream for the request.
func NewSignalStream(w http.ResponseWriter, r *http.Request, options ...SignalStreamOption) SignalStream {
	stream := SignalStream{sse: datastar.NewSSE(w, r)}
	for _, option := range options {
		if option != nil {
			option(&stream)
		}
	}
	return stream
}

// Redirect emits a Datastar redirect response for short-lived command handlers.
// Long-lived update streams should use SignalStream and Patch only.
func Redirect(w http.ResponseWriter, r *http.Request, location string) error {
	return datastar.NewSSE(w, r).Redirect(location)
}

// PatchResponse emits a single Datastar patch-signals response.
func PatchResponse(w http.ResponseWriter, r *http.Request, patch SignalPatch) error {
	return NewSignalStream(w, r).Patch(patch)
}

// Patch emits one Datastar patch-signals event. Empty patches are ignored.
func (s SignalStream) Patch(patch SignalPatch) error {
	if len(patch) == 0 {
		return nil
	}
	if s.trace == nil || s.streamID == "" {
		return s.writeForwarded(patch)
	}
	published := s.trace.Record(TraceRecord{
		StreamID: s.streamID, Stage: TraceStagePublished, Signals: patch, Origin: s.origin,
	})
	if err := s.writeForwarded(patch); err != nil {
		s.trace.Record(TraceRecord{
			StreamID: s.streamID, Stage: TraceStageDropped, Signals: patch,
			Sequence: published.Sequence, Origin: s.origin, Outcome: "write_error",
		})
		return err
	}
	s.trace.Record(TraceRecord{
		StreamID: s.streamID, Stage: TraceStageDelivered, Signals: patch,
		Sequence: published.Sequence, Origin: s.origin,
	})
	return nil
}

func (s SignalStream) writeForwarded(patch SignalPatch) error {
	if len(patch) == 0 {
		return nil
	}
	return s.sse.MarshalAndPatchSignals(patch)
}

// Forward relays signal patches published to streamID until ctx is canceled.
func (s SignalStream) Forward(ctx context.Context, broker *Broker, streamID string) error {
	if broker == nil || streamID == "" {
		return errMissingForwardTarget
	}
	updates, unsubscribe := broker.Subscribe(streamID)
	defer unsubscribe()
	return s.ForwardUpdates(ctx, updates)
}

// ForwardUpdates relays an already-subscribed mailbox. It lets callers
// subscribe before sending bootstrap state so no refresh event can be lost in
// the bootstrap-to-forward handoff.
func (s SignalStream) ForwardUpdates(ctx context.Context, updates <-chan SignalPatch) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case patch, ok := <-updates:
			if !ok {
				return nil
			}
			if err := s.writeForwarded(patch); err != nil {
				return err
			}
		}
	}
}

// Wait keeps a no-op stream open until ctx is canceled.
func (s SignalStream) Wait(ctx context.Context) {
	<-ctx.Done()
}
