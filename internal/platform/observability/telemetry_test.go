package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsExposeRuntimeHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	New().MetricsHandler("", nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, metric := range []string{"go_goroutines ", "process_resident_memory_bytes "} {
		if !strings.Contains(response.Body.String(), metric) {
			t.Errorf("metrics response missing %q", metric)
		}
	}
}
