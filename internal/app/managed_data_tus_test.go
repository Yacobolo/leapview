package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
)

func TestManagedDataTusRouteRejectsClientCreatedUploads(t *testing.T) {
	called := false
	server := assembleRuntime(fakeMetrics{}, testStoreOptions(testStore(t), assemblyConfig{

		ManagedDataTus: manageddatamodule.TusProtocolHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		})),
	}))

	request := httptest.NewRequest(http.MethodPost, "/upload-protocols/tus", nil)
	request.Header.Set("Authorization", "Bearer dev")
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if called {
		t.Fatal("tus backend received a client-created upload request")
	}
}

func TestManagedDataTusRouteForwardsResumableOperations(t *testing.T) {
	var method, path string
	server := assembleRuntime(fakeMetrics{}, testStoreOptions(testStore(t), assemblyConfig{

		ManagedDataTus: manageddatamodule.TusProtocolHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		})),
	}))

	request := httptest.NewRequest(http.MethodPatch, "/upload-protocols/tus/tus_abc", nil)
	request.Header.Set("Authorization", "Bearer dev")
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || method != http.MethodPatch || path != "/upload-protocols/tus/tus_abc" {
		t.Fatalf("status = %d, method = %q, path = %q", recorder.Code, method, path)
	}
}
