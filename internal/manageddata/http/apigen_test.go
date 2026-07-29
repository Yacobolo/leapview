package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	manageddataapi "github.com/flidai/leapview/internal/manageddata/api"
	manageddatagen "github.com/flidai/leapview/internal/manageddata/api/gen"
)

var _ manageddatagen.GenOperationDispatcher = (*APIGenDispatcher)(nil)
var _ manageddatagen.GenTransportErrorResponder = APIGenTransportErrorResponder{}

func TestAPIGenDispatcherMapsIdempotencyAndEventPaging(t *testing.T) {
	handler := &recordingAPIGenHandler{}
	dispatcher := NewAPIGenDispatcher(handler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/", nil)
	limit, pageToken, accept, lastEventID := int32(25), "page-1", "application/json", "event-9"

	dispatcher.CreateManagedDataUploadSession(recorder, request, "p1", "c1", manageddatagen.GenCreateManagedDataUploadSessionHeaders{IdempotencyKey: "create-1"})
	dispatcher.ListManagedDataUploadSessionEvents(
		recorder, request, "p1", "c1", "u1",
		manageddatagen.GenListManagedDataUploadSessionEventsParams{Limit: &limit, PageToken: &pageToken},
		manageddatagen.GenListManagedDataUploadSessionEventsHeaders{Accept: &accept, LastEventID: &lastEventID},
	)

	if got, want := handler.idempotencyKey, "create-1"; got != want {
		t.Fatalf("idempotency key = %q, want %q", got, want)
	}
	if got, want := *handler.eventPage.Limit, int32(25); got != want {
		t.Fatalf("event limit = %d, want %d", got, want)
	}
	if got, want := *handler.eventPage.PageToken, "page-1"; got != want {
		t.Fatalf("event page token = %q, want %q", got, want)
	}
	if got, want := *handler.eventHeaders.LastEventID, "event-9"; got != want {
		t.Fatalf("last event ID = %q, want %q", got, want)
	}
}

type recordingAPIGenHandler struct {
	APIGenHandler
	idempotencyKey string
	eventPage      manageddataapi.PageParams
	eventHeaders   manageddataapi.GenListManagedDataUploadSessionEventsHeaders
}

func (h *recordingAPIGenHandler) CreateManagedDataUploadSession(_ stdhttp.ResponseWriter, _ *stdhttp.Request, _, _ string, headers manageddataapi.IdempotencyHeaders) {
	h.idempotencyKey = headers.IdempotencyKey
}

func (h *recordingAPIGenHandler) ListManagedDataUploadSessionEvents(_ stdhttp.ResponseWriter, _ *stdhttp.Request, _, _, _ string, params manageddataapi.PageParams, headers manageddataapi.GenListManagedDataUploadSessionEventsHeaders) {
	h.eventPage, h.eventHeaders = params, headers
}
