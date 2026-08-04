package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sitehttp "github.com/flidai/leapview/internal/app/site/http"
)

func TestNewHandlerReturnsSiteServer(t *testing.T) {
	t.Parallel()

	var handler http.Handler = sitehttp.NewHandler()
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestBuildIdentityReportsTheServingSiteArtifact(t *testing.T) {
	t.Parallel()

	handler := sitehttp.NewHandlerWithOptions(sitehttp.Options{
		BuildRevision:  "0123456789abcdef0123456789abcdef01234567",
		ImageReference: "ghcr.io/flidai/leapview-site@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/build.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /build.json status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var identity struct {
		SchemaVersion int    `json:"schemaVersion"`
		Revision      string `json:"revision"`
		Image         string `json:"image"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode build identity: %v", err)
	}
	if identity.SchemaVersion != 1 || identity.Revision != "0123456789abcdef0123456789abcdef01234567" ||
		identity.Image != "ghcr.io/flidai/leapview-site@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("build identity = %#v", identity)
	}
}
