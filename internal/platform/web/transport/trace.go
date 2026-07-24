// Package transport owns shared UI and pagestream HTTP transport mechanics.
package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type TraceHandler struct{ Store *pagestream.TraceStore }

type traceResponse struct {
	Events    []pagestream.TraceEvent `json:"events"`
	NextAfter uint64                  `json:"nextAfter"`
}

type signalsResponse struct {
	StreamID  string                    `json:"streamId"`
	State     map[string]any            `json:"state"`
	Leaves    []pagestream.SignalLeaf   `json:"leaves"`
	History   []pagestream.SignalChange `json:"history"`
	NextAfter uint64                    `json:"nextAfter"`
}

func (h TraceHandler) Traces(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.NotFound(w, r)
		return
	}
	after, err := optionalUint64(r.URL.Query().Get("after"))
	if err != nil {
		http.Error(w, "after must be an unsigned integer", http.StatusBadRequest)
		return
	}
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	events := h.Store.Events(pagestream.TraceQuery{
		After: after, StreamID: strings.TrimSpace(r.URL.Query().Get("streamId")), Limit: limit,
	})
	nextAfter := after
	if len(events) > 0 {
		nextAfter = events[len(events)-1].ID
	}
	writeTraceJSON(w, traceResponse{Events: events, NextAfter: nextAfter})
}

func (h TraceHandler) Signals(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.NotFound(w, r)
		return
	}
	after, err := optionalUint64(r.URL.Query().Get("after"))
	if err != nil {
		http.Error(w, "after must be an unsigned integer", http.StatusBadRequest)
		return
	}
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		http.Error(w, "limit must be an integer", http.StatusBadRequest)
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path != "" && !strings.HasPrefix(path, "/") {
		http.Error(w, "path must be a JSON Pointer", http.StatusBadRequest)
		return
	}
	requestedStreamID := strings.TrimSpace(r.URL.Query().Get("streamId"))
	snapshot, ok := h.Store.SignalSnapshot(requestedStreamID)
	if !ok {
		snapshot = pagestream.SignalSnapshot{StreamID: requestedStreamID, State: map[string]any{}, Leaves: []pagestream.SignalLeaf{}}
	}
	history := h.Store.SignalChanges(pagestream.SignalHistoryQuery{
		After: after, StreamID: snapshot.StreamID, Path: path, Limit: limit,
	})
	nextAfter := after
	if len(history) > 0 {
		nextAfter = history[len(history)-1].ID
	}
	writeTraceJSON(w, signalsResponse{
		StreamID: snapshot.StreamID, State: snapshot.State, Leaves: snapshot.Leaves,
		History: history, NextAfter: nextAfter,
	})
}

func writeTraceJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func optionalUint64(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}

func optionalInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
