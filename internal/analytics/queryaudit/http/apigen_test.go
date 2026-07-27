package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	analyticsgen "github.com/Yacobolo/leapview/internal/analytics/api/gen"
)

var _ analyticsgen.GenOperationDispatcher = (*APIGenDispatcher)(nil)
var _ analyticsgen.GenTransportErrorResponder = APIGenTransportErrorResponder{}

func TestAPIGenDispatcherDelegatesQueryEventsToCapabilityHandler(t *testing.T) {
	called := false
	dispatcher := NewAPIGenDispatcher(queryEventsHandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		called = true
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()

	dispatcher.ListQueryEvents(
		recorder,
		httptest.NewRequest(stdhttp.MethodGet, "/api/v1/workspaces/sales/query-events?limit=10", nil),
		"sales",
		analyticsgen.GenListQueryEventsParams{},
	)

	if !called {
		t.Fatal("generated Analytics dispatcher did not delegate to the query-audit handler")
	}
	if got, want := recorder.Code, stdhttp.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

type queryEventsHandlerFunc func(stdhttp.ResponseWriter, *stdhttp.Request)

func (fn queryEventsHandlerFunc) ListQueryEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	fn(w, r)
}
