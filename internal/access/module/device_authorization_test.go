package module

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access"
	accesshttp "github.com/flidai/leapview/internal/access/http"
	"github.com/flidai/leapview/internal/platform/web/page"
)

type fakeBrowserAuthoringAuth struct {
	accesshttp.AuthoringAuthentication
	approvedCode string
	deniedCode   string
}

func (service *fakeBrowserAuthoringAuth) ApproveDeviceAuthorization(_ context.Context, principal access.Principal, code string) error {
	if principal.ID == "" || principal.Kind != access.PrincipalKindUser {
		return access.ErrInvalidAuthoringPrincipal
	}
	service.approvedCode = code
	return nil
}

func (service *fakeBrowserAuthoringAuth) DenyDeviceAuthorization(_ context.Context, principal access.Principal, code string) error {
	if principal.ID == "" {
		return access.ErrInvalidAuthoringPrincipal
	}
	service.deniedCode = code
	return nil
}

func TestDeviceAuthorizationBrowserFlowRendersAndApproves(t *testing.T) {
	service := &fakeBrowserAuthoringAuth{}
	module := &Module{
		handler:      accesshttp.Handler{AuthoringAuth: service},
		presentation: page.Presentation{ProductName: "LeapView"},
	}
	get := httptest.NewRequest(http.MethodGet, "/device?user_code=ABCD-EFGH", nil)
	getRecorder := httptest.NewRecorder()
	module.DeviceAuthorizationPage(getRecorder, get)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), "ABCD-EFGH") {
		t.Fatalf("GET status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	form := url.Values{"user_code": {"ABCD-EFGH"}, "decision": {"approve"}}
	post := httptest.NewRequest(http.MethodPost, "/device", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRecorder := httptest.NewRecorder()
	module.DeviceAuthorizationPage(postRecorder, post)
	if postRecorder.Code != http.StatusOK || service.approvedCode != "ABCD-EFGH" {
		t.Fatalf("POST status=%d approved=%q body=%s", postRecorder.Code, service.approvedCode, postRecorder.Body.String())
	}
	if !strings.Contains(postRecorder.Body.String(), "Return to your terminal") {
		t.Fatalf("result body=%s", postRecorder.Body.String())
	}
}

func TestDeviceAuthorizationBrowserFlowRejectsMissingCode(t *testing.T) {
	module := &Module{handler: accesshttp.Handler{AuthoringAuth: &fakeBrowserAuthoringAuth{}}}
	post := httptest.NewRequest(http.MethodPost, "/device", strings.NewReader("decision=approve"))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	module.DeviceAuthorizationPage(recorder, post)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
