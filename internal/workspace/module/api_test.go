package module

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWorkspaceRejectsUnavailableRepository(t *testing.T) {
	recorder := httptest.NewRecorder()
	(*Module)(nil).GetWorkspace(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/sales", nil), "sales")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
