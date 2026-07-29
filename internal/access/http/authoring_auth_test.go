package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/access"
)

type fakeAuthoringAuthentication struct {
	AuthoringAuthentication
	beginScope access.AuthoringScope
	approve    string
	exchange   func(string) (access.AuthoringTokenSet, error)
}

func (service *fakeAuthoringAuthentication) InstanceID() string { return "lvinst_prod" }

func (service *fakeAuthoringAuthentication) BeginDeviceAuthorization(_ context.Context, scope access.AuthoringScope) (access.DeviceAuthorizationResponse, error) {
	service.beginScope = scope
	return access.DeviceAuthorizationResponse{
		DeviceCode: "device-secret", UserCode: "ABCD-EFGH",
		VerificationURI:         "https://prod.example.com/device",
		VerificationURIComplete: "https://prod.example.com/device?user_code=ABCD-EFGH",
		ExpiresIn:               600, Interval: 5,
	}, nil
}

func (service *fakeAuthoringAuthentication) ApproveDeviceAuthorization(_ context.Context, _ access.Principal, userCode string) error {
	service.approve = userCode
	return nil
}

func (service *fakeAuthoringAuthentication) ExchangeDeviceCode(_ context.Context, code string) (access.AuthoringTokenSet, error) {
	return service.exchange(code)
}

func TestBeginDeviceAuthorizationReturnsNoStoreTypedPayload(t *testing.T) {
	service := &fakeAuthoringAuthentication{}
	handler := Handler{AuthoringAuth: service}
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/access/device-authorizations", strings.NewReader(
		`{"scope":{"projectId":"analytics","privileges":["DEPLOY","ACTIVATE_DEPLOYMENT"]}}`,
	))
	recorder := httptest.NewRecorder()
	handler.BeginDeviceAuthorization(recorder, request)
	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers = %#v", recorder.Header())
	}
	if service.beginScope.TargetID != "lvinst_prod" || service.beginScope.ProjectID != "analytics" ||
		len(service.beginScope.Privileges) != 2 {
		t.Fatalf("scope = %+v", service.beginScope)
	}
	for _, want := range []string{`"deviceCode":"device-secret"`, `"userCode":"ABCD-EFGH"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, recorder.Body.String())
		}
	}
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

func TestExchangeDeviceAuthorizationMapsPollingAndNeverCachesTokens(t *testing.T) {
	now := time.Now().UTC()
	calls := 0
	service := &fakeAuthoringAuthentication{exchange: func(code string) (access.AuthoringTokenSet, error) {
		calls++
		if code != "device-secret" {
			t.Fatalf("device code = %q", code)
		}
		if calls == 1 {
			return access.AuthoringTokenSet{}, access.ErrDeviceAuthorizationPending
		}
		return access.AuthoringTokenSet{
			AccessToken: "access-secret", RefreshToken: "refresh-secret", TokenType: "Bearer", ExpiresIn: 900,
			Session: access.AuthoringSession{
				ID: "session-1", Kind: access.AuthoringSessionHumanCLI, ClientID: access.AuthoringCLIClientID,
				Scope:     access.AuthoringScope{TargetID: "lvinst_prod", ProjectID: "analytics", Privileges: []access.Privilege{access.PrivilegeDeploy}},
				CreatedAt: now, ExpiresAt: now.Add(time.Hour),
			},
		}, nil
	}}
	handler := Handler{AuthoringAuth: service}
	request := func() *stdhttp.Request {
		return httptest.NewRequest(stdhttp.MethodPost, "/api/v1/access/device-authorizations/token", strings.NewReader(`{"deviceCode":"device-secret"}`))
	}
	pending := httptest.NewRecorder()
	handler.ExchangeDeviceAuthorization(pending, request())
	if pending.Code != stdhttp.StatusConflict {
		t.Fatalf("pending status = %d body=%s", pending.Code, pending.Body.String())
	}
	success := httptest.NewRecorder()
	handler.ExchangeDeviceAuthorization(success, request())
	if success.Code != stdhttp.StatusOK || success.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("success status=%d headers=%#v body=%s", success.Code, success.Header(), success.Body.String())
	}
	if !strings.Contains(success.Body.String(), "access-secret") || !strings.Contains(success.Body.String(), "refresh-secret") {
		t.Fatalf("token response = %s", success.Body.String())
	}
}

func TestExchangeDeviceAuthorizationRejectsUnknownFields(t *testing.T) {
	service := &fakeAuthoringAuthentication{exchange: func(string) (access.AuthoringTokenSet, error) {
		return access.AuthoringTokenSet{}, errors.New("must not be called")
	}}
	request := httptest.NewRequest(stdhttp.MethodPost, "/", strings.NewReader(`{"deviceCode":"device","token":"smuggled"}`))
	recorder := httptest.NewRecorder()
	(Handler{AuthoringAuth: service}).ExchangeDeviceAuthorization(recorder, request)
	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
