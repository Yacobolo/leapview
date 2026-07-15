package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/pkg/pagestream"
)

func TestDevelopmentPageStreamTraceEndpointReturnsSanitizedEvents(t *testing.T) {
	t.Setenv("LIBREDASH_PRODUCTION", "")
	var logs bytes.Buffer
	server := NewWithOptions(fakeMetrics{}, Options{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	})
	server.broker.PublishEnvelope("trace:test", pagestream.Envelope{
		Signals: pagestream.SignalPatch{"status": "ready", "token": "private"},
		Trace:   pagestream.TraceMetadata{Origin: "test.publisher"},
	})

	req := httptest.NewRequest(http.MethodGet, "/__dev/pagestream/traces?streamId=trace%3Atest", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("trace response = %d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	var response struct {
		Events    []pagestream.TraceEvent `json:"events"`
		NextAfter uint64                  `json:"nextAfter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode trace response: %v", err)
	}
	if len(response.Events) != 1 || response.NextAfter != response.Events[0].ID || response.Events[0].Payload["token"] != "[REDACTED]" {
		t.Fatalf("trace response = %#v", response)
	}
	if strings.Contains(logs.String(), "private") || !strings.Contains(logs.String(), "pagestream signal") {
		t.Fatalf("trace logs = %s", logs.String())
	}
}

func TestProductionOmitsPageStreamTraceEndpoint(t *testing.T) {
	t.Setenv("LIBREDASH_PRODUCTION", "1")
	server := New(fakeMetrics{})
	req := httptest.NewRequest(http.MethodGet, "/__dev/pagestream/traces", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("production trace status = %d, want 404", rec.Code)
	}
}
