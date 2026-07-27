package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneralRendersAdminOwnedPageAdapter(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)

	Handler{ReadModel: ReadModel{}}.General(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "<lv-admin-page") || !strings.Contains(body, "route=admin") {
		t.Fatalf("admin handler did not render the admin route shell:\n%s", body)
	}
}
