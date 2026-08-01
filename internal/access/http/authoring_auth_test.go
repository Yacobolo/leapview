package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access"
)

type fakeAuthoringAuthentication struct {
	AuthoringAuthentication
	approve string
}

func (service *fakeAuthoringAuthentication) InstanceID() string { return "lvinst_prod" }

func (service *fakeAuthoringAuthentication) ApproveDeviceAuthorization(_ context.Context, _ access.Principal, userCode string) error {
	service.approve = userCode
	return nil
}

func TestDeviceAuthorizationApprovalRejectsBearerCredentials(t *testing.T) {
	service := &fakeAuthoringAuthentication{}
	handler := Handler{
		AuthoringAuth: service,
		CurrentPrincipal: func(*stdhttp.Request) (Principal, bool) {
			return Principal{ID: "principal-1", Email: "developer@example.com"}, true
		},
		CurrentCredential: func(*stdhttp.Request) (access.APICredential, bool) {
			return access.APICredential{}, true
		},
	}
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/access/device-authorizations/approval", strings.NewReader(
		`{"userCode":"ABCD-EFGH","approved":true}`,
	))
	recorder := httptest.NewRecorder()
	handler.DecideDeviceAuthorization(recorder, request)
	if recorder.Code != stdhttp.StatusForbidden || service.approve != "" {
		t.Fatalf("status=%d approved=%q body=%s", recorder.Code, service.approve, recorder.Body.String())
	}
}
