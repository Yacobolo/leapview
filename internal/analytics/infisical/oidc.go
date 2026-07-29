package infisical

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

type IdentityTokenSource interface {
	IdentityToken(context.Context) (string, error)
}

type OIDCAuthConfig struct {
	BaseURL        string
	IdentityID     string
	IdentityTokens IdentityTokenSource `json:"-" yaml:"-"`
	HTTPClient     *http.Client
	Now            func() time.Time
}

func (OIDCAuthConfig) String() string   { return "<infisical-oidc-auth-config:redacted>" }
func (OIDCAuthConfig) GoString() string { return "infisical.OIDCAuthConfig{<redacted>}" }

type OIDCAuthenticator struct {
	config  OIDCAuthConfig
	baseURL *url.URL

	mu    sync.Mutex
	token AccessToken
}

func NewOIDCAuthenticator(config OIDCAuthConfig) (*OIDCAuthenticator, error) {
	baseURL, err := validateBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	config.IdentityID = strings.TrimSpace(config.IdentityID)
	if config.IdentityID == "" || config.IdentityTokens == nil || config.HTTPClient == nil || config.Now == nil {
		return nil, fmt.Errorf("%w: OIDC identity, workload token source, HTTP client, and clock are required", connectionbinding.ErrInvalidBinding)
	}
	config.HTTPClient = hardenedClient(config.HTTPClient)
	return &OIDCAuthenticator{config: config, baseURL: baseURL}, nil
}

func (*OIDCAuthenticator) String() string   { return "<infisical-oidc-authenticator:redacted>" }
func (*OIDCAuthenticator) GoString() string { return "infisical.OIDCAuthenticator{<redacted>}" }

func (authenticator *OIDCAuthenticator) AccessToken(ctx context.Context) (AccessToken, error) {
	if authenticator == nil {
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	authenticator.mu.Lock()
	defer authenticator.mu.Unlock()
	now := authenticator.config.Now().UTC()
	if authenticator.token.value != "" && now.Before(authenticator.token.refreshAt) {
		return authenticator.token, nil
	}
	identityToken, err := authenticator.config.IdentityTokens.IdentityToken(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return AccessToken{}, err
		}
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	if strings.TrimSpace(identityToken) == "" {
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	endpoint := *authenticator.baseURL
	endpoint.Path = "/api/v1/auth/oidc-auth/login"
	form := url.Values{"identityId": {authenticator.config.IdentityID}, "jwt": {identityToken}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := authenticator.config.HTTPClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return AccessToken{}, err
		}
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return AccessToken{}, statusError(response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return AccessToken{}, connectionbinding.ErrProviderUnavailable
	}
	token, err := decodeAccessToken(body, now)
	if err != nil {
		return AccessToken{}, err
	}
	authenticator.token = token
	return token, nil
}
