package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	webpage "github.com/flidai/leapview/internal/platform/web/page"
)

func TestServeCandidatePreviewDelegatesOpaqueScopeToDeploymentModule(t *testing.T) {
	handler := &candidatePreviewHandlerStub{}
	request := httptest.NewRequest(http.MethodGet, "/candidates/cand_opaque", nil)
	response := httptest.NewRecorder()

	serveCandidatePreview(handler, "cand_opaque", "principal_1", nil, response, request)

	if !handler.called || handler.candidateID != "cand_opaque" || handler.principalID != "principal_1" {
		t.Fatalf("candidate preview delegation = %#v", handler)
	}
}

func TestServeCandidatePreviewRejectsIncompleteScope(t *testing.T) {
	handler := &candidatePreviewHandlerStub{}
	request := httptest.NewRequest(http.MethodGet, "/candidates/", nil)
	response := httptest.NewRecorder()

	serveCandidatePreview(handler, "", "principal_1", nil, response, request)

	if response.Code != http.StatusNotFound || handler.called {
		t.Fatalf("incomplete preview status=%d delegated=%t", response.Code, handler.called)
	}
}

type candidatePreviewHandlerStub struct {
	called                   bool
	candidateID, principalID string
}

func (stub *candidatePreviewHandlerStub) ServeCandidatePreview(
	_ http.ResponseWriter,
	_ *http.Request,
	candidateID, principalID string,
	_ webpage.Provider,
) {
	stub.called = true
	stub.candidateID = candidateID
	stub.principalID = principalID
}
