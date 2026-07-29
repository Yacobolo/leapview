package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flidai/leapview/internal/app/desktopdiscovery"
)

func TestDesktopDiscoveryIsPublicAndMountedAtWellKnownPath(t *testing.T) {
	server := assembleRuntime(fakeMetrics{}, assemblyConfig{
		DesktopDiscovery: desktopdiscovery.Config{
			CanonicalOrigin: "https://analytics.company.com",
			InstanceID:      "instance_0123456789abcdef0123456789abcdef",
			DisplayName:     "Company Analytics",
			ServerVersion:   "v1.4.2",
		},
	})
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, desktopdiscovery.WellKnownPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var document desktopdiscovery.Document
	if err := json.NewDecoder(recorder.Body).Decode(&document); err != nil {
		t.Fatalf("decode discovery document: %v", err)
	}
	if document.CanonicalOrigin != "https://analytics.company.com" ||
		document.InstanceID != "instance_0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected discovery document: %#v", document)
	}
}
