package module

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access"
	accesshttp "github.com/flidai/leapview/internal/access/http"
)

type fakeAuthoringOAuth struct {
	accesshttp.AuthoringAuthentication
	beganScope    access.AuthoringScope
	exchangedCode string
	exchangeErr   error
}

func (service *fakeAuthoringOAuth) InstanceID() string {
	return "lvinst_prod"
}

func (service *fakeAuthoringOAuth) BeginDeviceAuthorization(_ context.Context, scope access.AuthoringScope) (access.DeviceAuthorizationResponse, error) {
	service.beganScope = scope
	return access.DeviceAuthorizationResponse{
		DeviceCode:              "device-secret",
		UserCode:                "ABCD-EFGH",
		VerificationURI:         "https://prod.example.com/device",
		VerificationURIComplete: "https://prod.example.com/device?user_code=ABCD-EFGH",
		ExpiresIn:               600,
		Interval:                5,
	}, nil
}

func (service *fakeAuthoringOAuth) ExchangeDeviceCode(_ context.Context, code string) (access.AuthoringTokenSet, error) {
	service.exchangedCode = code
	if service.exchangeErr != nil {
		return access.AuthoringTokenSet{}, service.exchangeErr
	}
	return access.AuthoringTokenSet{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		TokenType:    "Bearer",
		ExpiresIn:    900,
		Session: access.AuthoringSession{
			ID:       "session-1",
			Kind:     access.AuthoringSessionHumanCLI,
			ClientID: access.AuthoringCLIClientID,
			Scope: access.AuthoringScope{
				TargetID:   "lvinst_prod",
				ProjectID:  "analytics",
				Privileges: []access.Privilege{access.PrivilegeDeploy},
			},
		},
	}, nil
}

func TestAuthoringDeviceAuthorizationUsesRFC8628WireFormat(t *testing.T) {
	service := &fakeAuthoringOAuth{}
	module := &Module{handler: accesshttp.Handler{AuthoringAuth: service}}
	form := url.Values{
		"client_id":  {access.AuthoringCLIClientID},
		"project_id": {"analytics"},
		"scope":      {string(access.PrivilegeDeploy)},
	}
	request := httptest.NewRequest(http.MethodPost, "/oauth/device/code", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	module.AuthoringDeviceAuthorization(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers=%v", recorder.Header())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["device_code"] != "device-secret" ||
		response["user_code"] != "ABCD-EFGH" ||
		response["verification_uri"] != "https://prod.example.com/device" ||
		response["verification_uri_complete"] != "https://prod.example.com/device?user_code=ABCD-EFGH" ||
		response["expires_in"] != float64(600) ||
		response["interval"] != float64(5) {
		t.Fatalf("response=%v", response)
	}
	if service.beganScope.TargetID != "lvinst_prod" ||
		service.beganScope.ProjectID != "analytics" ||
		len(service.beganScope.Privileges) != 1 ||
		service.beganScope.Privileges[0] != access.PrivilegeDeploy {
		t.Fatalf("scope=%+v", service.beganScope)
	}
}

func TestAuthoringDeviceTokenUsesOAuthErrorsAndTokenFields(t *testing.T) {
	for name, test := range map[string]struct {
		err       error
		status    int
		errorCode string
	}{
		"pending": {
			err:       access.ErrDeviceAuthorizationPending,
			status:    http.StatusBadRequest,
			errorCode: "authorization_pending",
		},
		"slow down": {
			err:       access.ErrDeviceAuthorizationSlowDown,
			status:    http.StatusBadRequest,
			errorCode: "slow_down",
		},
		"denied": {
			err:       access.ErrDeviceAuthorizationDenied,
			status:    http.StatusBadRequest,
			errorCode: "access_denied",
		},
		"expired": {
			err:       access.ErrDeviceAuthorizationExpired,
			status:    http.StatusBadRequest,
			errorCode: "expired_token",
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := &fakeAuthoringOAuth{exchangeErr: test.err}
			module := &Module{handler: accesshttp.Handler{AuthoringAuth: service}}
			form := url.Values{
				"client_id":   {access.AuthoringCLIClientID},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {"device-secret"},
			}
			request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()

			module.AuthoringOAuthToken(recorder, request)

			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != test.errorCode {
				t.Fatalf("error=%q want=%q body=%s", response.Error, test.errorCode, recorder.Body.String())
			}
		})
	}

	service := &fakeAuthoringOAuth{}
	module := &Module{handler: accesshttp.Handler{AuthoringAuth: service}}
	form := url.Values{
		"client_id":   {access.AuthoringCLIClientID},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {"device-secret"},
	}
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	module.AuthoringOAuthToken(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers=%v", recorder.Header())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["access_token"] != "access-secret" ||
		response["refresh_token"] != "refresh-secret" ||
		response["token_type"] != "Bearer" ||
		response["expires_in"] != float64(900) ||
		response["session_id"] != "session-1" ||
		response["target_id"] != "lvinst_prod" ||
		response["project_id"] != "analytics" {
		t.Fatalf("response=%v", response)
	}
	if service.exchangedCode != "device-secret" {
		t.Fatalf("device code=%q", service.exchangedCode)
	}
}

func TestAuthoringOAuthRoutingIsUnambiguous(t *testing.T) {
	for name, values := range map[string]url.Values{
		"device grant": {
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":  {access.AuthoringCLIClientID},
		},
		"authoring client": {
			"grant_type": {"refresh_token"},
			"client_id":  {access.AuthoringCLIClientID},
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(values.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if !requestTargetsAuthoringOAuth(request) {
				t.Fatal("request was not routed to authoring OAuth")
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(url.Values{
		"grant_type": {"authorization_code"},
		"client_id":  {"mcp-client"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if requestTargetsAuthoringOAuth(request) {
		t.Fatal("MCP authorization-code request routed to authoring OAuth")
	}
}
