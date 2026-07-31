package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/access"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const authoringDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

type DeviceAuthorizationRequest struct {
	Origin     string
	ProjectID  string
	Privileges []string
}

type OAuthRefreshRequest struct {
	Origin       string
	RefreshToken string
}

type DeviceAuthorization interface {
	Challenge() DeviceChallenge
	Token(context.Context) (*oauth2.Token, error)
}

type AuthoringOAuthClient interface {
	Begin(context.Context, DeviceAuthorizationRequest) (DeviceAuthorization, error)
	Refresh(context.Context, OAuthRefreshRequest) (*oauth2.Token, error)
	Workload(context.Context, WorkloadIdentityRequest) (*oauth2.Token, error)
}

type StandardOAuthClient struct {
	HTTPClient *http.Client
}

func (client StandardOAuthClient) Begin(ctx context.Context, request DeviceAuthorizationRequest) (DeviceAuthorization, error) {
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
		client.context(ctx),
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
		client:   client,
		config:   config,
		response: response,
		challenge: DeviceChallenge{
			UserCode:                response.UserCode,
			VerificationURI:         response.VerificationURI,
			VerificationURIComplete: response.VerificationURIComplete,
			ExpiresIn:               expiresIn,
		},
	}, nil
}

func (client StandardOAuthClient) Refresh(ctx context.Context, request OAuthRefreshRequest) (*oauth2.Token, error) {
	origin := strings.TrimRight(strings.TrimSpace(request.Origin), "/")
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if origin == "" || refreshToken == "" {
		return nil, fmt.Errorf("OAuth refresh origin and credential are required")
	}
	config := oauth2.Config{
		ClientID: access.AuthoringCLIClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL:  origin + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	token, err := config.TokenSource(client.context(ctx), &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh OAuth credential: %w", err)
	}
	return token, nil
}

func (client StandardOAuthClient) Workload(ctx context.Context, request WorkloadIdentityRequest) (*oauth2.Token, error) {
	origin := strings.TrimRight(strings.TrimSpace(request.Origin), "/")
	clientID := strings.TrimSpace(request.ClientID)
	if origin == "" || strings.TrimSpace(request.ProjectID) == "" ||
		clientID == "" || strings.TrimSpace(request.ClientSecret) == "" ||
		len(request.Privileges) == 0 || request.Lifetime < time.Second {
		return nil, fmt.Errorf("OAuth workload origin, project, client credentials, privileges, and lifetime are required")
	}
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: request.ClientSecret,
		TokenURL:     origin + "/oauth/token",
		Scopes:       append([]string(nil), request.Privileges...),
		AuthStyle:    oauth2.AuthStyleInParams,
		EndpointParams: url.Values{
			"project_id":       {request.ProjectID},
			"lifetime_seconds": {strconv.FormatInt(int64(request.Lifetime/time.Second), 10)},
		},
	}
	token, err := config.Token(client.context(ctx))
	if err != nil {
		return nil, fmt.Errorf("exchange OAuth client credentials: %w", err)
	}
	return token, nil
}

func (client StandardOAuthClient) context(ctx context.Context) context.Context {
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return context.WithValue(ctx, oauth2.HTTPClient, httpClient)
}

type oauthDeviceAuthorization struct {
	client    StandardOAuthClient
	config    oauth2.Config
	response  *oauth2.DeviceAuthResponse
	challenge DeviceChallenge
}

func (authorization *oauthDeviceAuthorization) Challenge() DeviceChallenge {
	return authorization.challenge
}

func (authorization *oauthDeviceAuthorization) Token(ctx context.Context) (*oauth2.Token, error) {
	token, err := authorization.config.DeviceAccessToken(
		authorization.client.context(ctx),
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
	details, err := oauthAuthoringTokenSessionDetails(token)
	if err != nil || strings.TrimSpace(token.RefreshToken) == "" {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth authorization returned incomplete credentials")
	}
	if details.Kind != access.AuthoringSessionHumanCLI {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth authorization returned unexpected session kind")
	}
	return details, nil
}

func oauthWorkloadTokenDetails(token *oauth2.Token) (authoringOAuthTokenDetails, error) {
	details, err := oauthAuthoringTokenSessionDetails(token)
	if err != nil || strings.TrimSpace(token.RefreshToken) != "" {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth workload exchange returned incomplete credentials")
	}
	if details.Kind != access.AuthoringSessionWorkload {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth workload exchange returned unexpected session kind")
	}
	return details, nil
}

func oauthAuthoringTokenSessionDetails(token *oauth2.Token) (authoringOAuthTokenDetails, error) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" || token.Expiry.IsZero() {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth authorization returned incomplete credentials")
	}
	details := authoringOAuthTokenDetails{
		SessionID: oauthTokenString(token, "session_id"),
		Kind:      access.AuthoringSessionKind(oauthTokenString(token, "session_kind")),
		TargetID:  oauthTokenString(token, "target_id"),
		ProjectID: oauthTokenString(token, "project_id"),
	}
	if details.SessionID == "" || details.Kind == "" || details.TargetID == "" || details.ProjectID == "" {
		return authoringOAuthTokenDetails{}, fmt.Errorf("OAuth authorization returned incomplete session metadata")
	}
	return details, nil
}

func oauthTokenString(token *oauth2.Token, name string) string {
	value, _ := token.Extra(name).(string)
	return strings.TrimSpace(value)
}
