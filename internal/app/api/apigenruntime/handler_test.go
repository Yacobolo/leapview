package apigenruntime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testAuthorizer struct{}

func (testAuthorizer) Protect(_ string, next http.Handler) (http.Handler, bool) {
	return next, true
}

func TestHandlerUsesInjectedGeneratedPartitionDispatch(t *testing.T) {
	called := false
	handler, err := Build(testAuthorizer{}, func(operationID string, w http.ResponseWriter, _ *http.Request) bool {
		called = true
		if operationID != "getAgentConversation" {
			t.Fatalf("operation ID = %q", operationID)
		}
		w.WriteHeader(http.StatusAccepted)
		return true
	})
	if err != nil {
		t.Fatalf("build handler: %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.HandleAPIGen("getAgentConversation", recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("partition dispatch was not called")
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestBuildRejectsMissingDispatch(t *testing.T) {
	if _, err := Build(testAuthorizer{}, nil); err == nil {
		t.Fatal("Build accepted a nil dispatch function")
	}
}
