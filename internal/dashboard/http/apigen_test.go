package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	dashboardgen "github.com/flidai/leapview/internal/dashboard/api/gen"
)

var _ dashboardgen.GenOperationDispatcher = (*APIGenDispatcher)(nil)
var _ dashboardgen.GenTransportErrorResponder = APIGenTransportErrorResponder{}

func TestAPIGenDispatcherForwardsWorkspaceScopedOperations(t *testing.T) {
	handler := &recordingAPIGenHandler{}
	dispatcher := NewAPIGenDispatcher(handler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/", nil)

	dispatcher.QuerySemanticModel(recorder, request, "sales", "orders", dashboardgen.GenQuerySemanticModelHeaders{})
	dispatcher.RotateDashboardPublication(recorder, request, "operations", "public-board", dashboardgen.GenRotateDashboardPublicationHeaders{})

	if got, want := handler.queryWorkspace, "sales"; got != want {
		t.Fatalf("query workspace = %q, want %q", got, want)
	}
	if gotWorkspace, gotPublication := handler.publicationWorkspace, handler.publicationName; gotWorkspace != "operations" || gotPublication != "public-board" {
		t.Fatalf("publication scope = %q/%q, want operations/public-board", gotWorkspace, gotPublication)
	}
}

type recordingAPIGenHandler struct {
	APIGenHandler
	queryWorkspace       string
	publicationWorkspace string
	publicationName      string
}

func (h *recordingAPIGenHandler) QuerySemanticModel(_ stdhttp.ResponseWriter, _ *stdhttp.Request, workspace string) {
	h.queryWorkspace = workspace
}

func (h *recordingAPIGenHandler) RotateDashboardPublication(_ stdhttp.ResponseWriter, _ *stdhttp.Request, workspace, publication string) {
	h.publicationWorkspace, h.publicationName = workspace, publication
}
