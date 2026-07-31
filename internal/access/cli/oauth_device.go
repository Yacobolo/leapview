package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/access"
	"golang.org/x/oauth2"
)

const authoringDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

type DeviceAuthorizationRequest struct {
	Origin     string
	ProjectID  string
	Privileges []string
}

type DeviceAuthorization interface {
	Challenge() DeviceChallenge
	Token(context.Context) (*oauth2.Token, error)
}

type DeviceAuthorizer interface {
	Begin(context.Context, DeviceAuthorizationRequest) (DeviceAuthorization, error)
}

type OAuthDeviceAuthorizer struct {
	HTTPClient *http.Client
}

func (authorizer OAuthDeviceAuthorizer) Begin(ctx context.Context, request DeviceAuthorizationRequest) (DeviceAuthorization, error) {
	origin := strings.TrimRight(strings.TrimSpace(request.Origin), "/")
	if origin == "" || strings.TrimSpace(request.ProjectID) == "" || len(request.Privileges) == 0 {
		return nil, fmt.Errorf("device authorization origin, project, and privileges are required")
	}
	config := oauth2.Config{
		ClientID: access.AuthoringCLIClientID,
		Scopes:   append([]string(nil), request.Privileges...),
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: origin + "/oauth/device/code",
			TokenURL:      origin + "/oauth/token",
			AuthStyle:     oauth2.AuthStyleInParams,
		},
	}
	response, err := config.DeviceAuth(
		authorizer.context(ctx),
		oauth2.SetAuthURLParam("project_id", request.ProjectID),
	)
	if err != nil {
		return nil, fmt.Errorf("begin OAuth device authorization: %w", err)
	}
	expiresIn := time.Until(response.Expiry)
	if expiresIn < 0 {
		expiresIn = 0
	}
	return &oauthDeviceAuthorization{
		authorizer: authorizer,
		config:     config,
		response:   response,
		challenge: DeviceChallenge{
			UserCode:                response.UserCode,
			VerificationURI:         response.VerificationURI,
			VerificationURIComplete: response.VerificationURIComplete,
			ExpiresIn:               expiresIn,
		},
	}, nil
}

func (authorizer OAuthDeviceAuthorizer) context(ctx context.Context) context.Context {
	client := authorizer.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}

type oauthDeviceAuthorization struct {
	authorizer OAuthDeviceAuthorizer
	config     oauth2.Config
	response   *oauth2.DeviceAuthResponse
	challenge  DeviceChallenge
}

func (authorization *oauthDeviceAuthorization) Challenge() DeviceChallenge {
	return authorization.challenge
}

func (authorization *oauthDeviceAuthorization) Token(ctx context.Context) (*oauth2.Token, error) {
	token, err := authorization.config.DeviceAccessToken(
		authorization.authorizer.context(ctx),
		authorization.response,
	)
	if err != nil {
		return nil, fmt.Errorf("exchange OAuth device authorization: %w", err)
	}
	return token, nil
}

type authoringOAuthTokenDetails struct {
	SessionID string
	Kind      access.AuthoringSessionKind
	TargetID  string
	ProjectID string
}

func oauthAuthoringTokenDetails(token *oauth2.Token) (authoringOAuthTokenDetails, error) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" || strings.TrimSpace(token.RefreshToken) == "" {
		return authoringOAuthTokenDetails{}, fmt.Errorf("device authorization returned incomplete credentials")
	}
	details := authoringOAuthTokenDetails{
		SessionID: oauthTokenString(token, "session_id"),
		Kind:      access.AuthoringSessionKind(oauthTokenString(token, "session_kind")),
		TargetID:  oauthTokenString(token, "target_id"),
		ProjectID: oauthTokenString(token, "project_id"),
	}
	if details.SessionID == "" || details.Kind != access.AuthoringSessionHumanCLI ||
		details.TargetID == "" || details.ProjectID == "" || token.Expiry.IsZero() {
		return authoringOAuthTokenDetails{}, fmt.Errorf("device authorization returned incomplete session metadata")
	}
	return details, nil
}

func oauthTokenString(token *oauth2.Token, name string) string {
	value, _ := token.Extra(name).(string)
	return strings.TrimSpace(value)
}
