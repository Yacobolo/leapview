package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	accesscli "github.com/flidai/leapview/internal/access/cli"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type fakeOriginResolver struct {
	calls int
}

func (resolver *fakeOriginResolver) ResolveOrigin(_ context.Context, origin, token string) (accesscli.ResolvedCredential, error) {
	resolver.calls++
	return accesscli.ResolvedCredential{AccessToken: "lv_cli_access_refreshed"}, nil
}

func TestAuthoringRetryTransportRefreshesOnceAfterMidSyncExpiry(t *testing.T) {
	requests := 0
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		status := http.StatusUnauthorized
		if token == "lv_cli_access_refreshed" {
			status = http.StatusOK
		}
		return &http.Response{
			StatusCode: status, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(http.StatusText(status))), Request: request,
		}, nil
	})
	resolver := &fakeOriginResolver{}
	transport := authoringRetryTransport{base: base, credentials: resolver}
	request, err := http.NewRequest(http.MethodPost, "https://prod.example.com/api/v1/projects/analytics/releases", strings.NewReader(`{}`))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer lv_cli_access_expired")
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || requests != 2 || resolver.calls != 1 {
		t.Fatalf("status=%d requests=%d refreshes=%d", response.StatusCode, requests, resolver.calls)
	}
}

func TestAuthoringRetryTransportNeverSubstitutesForPATOrWorkloadCredential(t *testing.T) {
	for _, token := range []string{"legacy-pat", "lv_workload_access_ephemeral"} {
		t.Run(token, func(t *testing.T) {
			resolver := &fakeOriginResolver{}
			transport := authoringRetryTransport{
				base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusUnauthorized, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader("unauthorized")), Request: request,
					}, nil
				}),
				credentials: resolver,
			}
			request, _ := http.NewRequest(http.MethodGet, "https://prod.example.com/api/v1/instance", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response, err := transport.RoundTrip(request)
			require.NoError(t, err)
			response.Body.Close()
			if resolver.calls != 0 {
				t.Fatalf("resolver calls = %d", resolver.calls)
			}
		})
	}
}
