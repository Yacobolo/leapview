package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	accesscli "github.com/flidai/leapview/internal/access/cli"
)

type originCredentialResolver interface {
	ResolveOrigin(context.Context, string, string) (accesscli.ResolvedCredential, error)
}

type authoringRetryTransport struct {
	base        http.RoundTripper
	credentials originCredentialResolver
}

func (transport authoringRetryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	response, err := base.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusUnauthorized || transport.credentials == nil {
		return response, err
	}
	currentToken := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(currentToken, "lv_cli_access_") {
		return response, nil
	}
	origin := (&url.URL{Scheme: request.URL.Scheme, Host: request.URL.Host}).String()
	resolved, resolveErr := transport.credentials.ResolveOrigin(request.Context(), origin, currentToken)
	if resolveErr != nil {
		response.Body.Close()
		return nil, fmt.Errorf("refresh authoring credential after HTTP 401: %w", resolveErr)
	}
	if resolved.AccessToken == "" || resolved.AccessToken == currentToken {
		return response, nil
	}
	retry := request.Clone(request.Context())
	retry.Header = request.Header.Clone()
	retry.Header.Set("Authorization", "Bearer "+resolved.AccessToken)
	if request.Body != nil {
		if request.GetBody == nil {
			return response, nil
		}
		body, bodyErr := request.GetBody()
		if bodyErr != nil {
			response.Body.Close()
			return nil, bodyErr
		}
		retry.Body = body
	}
	response.Body.Close()
	return base.RoundTrip(retry)
}

type applicationOriginCredentials struct {
	client *http.Client
}

func (resolver applicationOriginCredentials) ResolveOrigin(ctx context.Context, origin, accessToken string) (accesscli.ResolvedCredential, error) {
	authentication, err := defaultAuthoringAuthenticator(resolver.client)
	if err != nil {
		return accesscli.ResolvedCredential{}, err
	}
	return authentication.ResolveOrigin(ctx, origin, accessToken)
}

func authoringRefreshingHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clone := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	plain := *client
	plain.Transport = base
	clone.Transport = authoringRetryTransport{
		base: base, credentials: applicationOriginCredentials{client: &plain},
	}
	return &clone
}
