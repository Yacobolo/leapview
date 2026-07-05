package pagestream

import (
	"context"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

type StreamSpec struct {
	Broker         *Broker
	StreamID       string
	InitialPatches []Patch
	Snapshot       func(context.Context) []Patch
	TickerInterval time.Duration
}

func ServeStream(w http.ResponseWriter, r *http.Request, spec StreamSpec) {
	sse := datastar.NewSSE(w, r)
	patchAll := func(patches []Patch) bool {
		for _, patch := range patches {
			if len(patch) == 0 {
				continue
			}
			if err := sse.MarshalAndPatchSignals(patch); err != nil {
				return false
			}
		}
		return true
	}
	if !patchAll(spec.InitialPatches) {
		return
	}
	if spec.Snapshot != nil && !patchAll(spec.Snapshot(r.Context())) {
		return
	}
	var tickerC <-chan time.Time
	if spec.Snapshot != nil && spec.TickerInterval > 0 {
		ticker := time.NewTicker(spec.TickerInterval)
		defer ticker.Stop()
		tickerC = ticker.C
	}
	if spec.Broker == nil || spec.StreamID == "" {
		for {
			select {
			case <-r.Context().Done():
				return
			case <-tickerC:
				if !patchAll(spec.Snapshot(r.Context())) {
					return
				}
			}
		}
	}
	updates, unsubscribe := spec.Broker.Subscribe(spec.StreamID)
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case patch, ok := <-updates:
			if !ok {
				return
			}
			if err := sse.MarshalAndPatchSignals(patch); err != nil {
				return
			}
		case <-tickerC:
			if !patchAll(spec.Snapshot(r.Context())) {
				return
			}
		}
	}
}
