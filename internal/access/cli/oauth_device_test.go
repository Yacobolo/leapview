package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access"
)

func TestOAuthDeviceAuthorizerUsesRFC8628Client(t *testing.T) {
	var deviceRequest, tokenRequest url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		switch r.URL.Path {
		case "/oauth/device/code":
			deviceRequest = r.Form
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "device-secret",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          serverURL(r) + "/device",
				"verification_uri_complete": serverURL(r) + "/device?user_code=ABCD-EFGH",
				"expires_in":                60,
				"interval":                  1,
			})
		case "/oauth/token":
			tokenRequest = r.Form
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-secret",
				"refresh_token": "refresh-secret",
				"token_type":    "Bearer",
				"expires_in":    900,
				"session_id":    "session-1",
				"session_kind":  "human_cli",
				"target_id":     "lvinst_prod",
				"project_id":    "analytics",
				"scope":         "DEPLOY ACTIVATE_DEPLOYMENT",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	authorizer := OAuthDeviceAuthorizer{HTTPClient: server.Client()}
	authorization, err := authorizer.Begin(context.Background(), DeviceAuthorizationRequest{
		Origin:     server.URL,
		ProjectID:  "analytics",
		Privileges: []string{"DEPLOY", "ACTIVATE_DEPLOYMENT"},
	})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	challenge := authorization.Challenge()
	if challenge.UserCode != "ABCD-EFGH" || !strings.HasSuffix(challenge.VerificationURI, "/device") {
		t.Fatalf("challenge = %+v", challenge)
	}
	token, err := authorization.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "access-secret" || token.RefreshToken != "refresh-secret" {
		t.Fatalf("token = %+v", token)
	}
	details, err := oauthAuthoringTokenDetails(token)
	if err != nil {
		t.Fatalf("oauthAuthoringTokenDetails() error = %v", err)
	}
	if details.SessionID != "session-1" ||
		details.Kind != access.AuthoringSessionHumanCLI ||
		details.TargetID != "lvinst_prod" ||
		details.ProjectID != "analytics" {
		t.Fatalf("details = %+v", details)
	}
	if deviceRequest.Get("client_id") != access.AuthoringCLIClientID ||
		deviceRequest.Get("project_id") != "analytics" ||
		deviceRequest.Get("scope") != "DEPLOY ACTIVATE_DEPLOYMENT" {
		t.Fatalf("device request = %v", deviceRequest)
	}
	if tokenRequest.Get("grant_type") != authoringDeviceGrantType ||
		tokenRequest.Get("client_id") != access.AuthoringCLIClientID ||
		tokenRequest.Get("device_code") != "device-secret" {
		t.Fatalf("token request = %v", tokenRequest)
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
