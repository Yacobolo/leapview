package transport

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type Authorize func(route, section string, next http.Handler) (http.Handler, bool)

type PageStreamConfig struct {
	Trace     *pagestream.TraceStore
	Handlers  map[string]http.Handler
	Authorize Authorize
}

type PageStream struct {
	config PageStreamConfig
}

func NewPageStream(config PageStreamConfig) *PageStream {
	return &PageStream{config: config}
}

func (s *PageStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := Route(r)
	if route == "" {
		http.Error(w, "updates route is required", http.StatusBadRequest)
		return
	}
	handler := s.config.Handlers[route]
	if handler == nil {
		http.Error(w, "unknown updates route", http.StatusBadRequest)
		return
	}
	if s.config.Authorize == nil {
		handler.ServeHTTP(w, r)
		return
	}
	authorized, ok := s.config.Authorize(route, r.URL.Query().Get("section"), handler)
	if !ok || authorized == nil {
		http.Error(w, "unknown updates route", http.StatusBadRequest)
		return
	}
	authorized.ServeHTTP(w, r)
}

func Route(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.URL.Query().Get("route"))
}

func PatchAndWait(trace *pagestream.TraceStore, w http.ResponseWriter, r *http.Request, patch pagestream.SignalPatch) {
	clientID := pagestream.EnsureClientID(w, r)
	route := Route(r)
	streamID := route + ":" + clientID
	updates := pagestream.NewSignalStream(w, r, pagestream.WithStreamTrace(trace, streamID, route+".bootstrap"))
	if err := updates.Patch(patch); err != nil {
		return
	}
	updates.Wait(r.Context())
}
