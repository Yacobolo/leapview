package infisical

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	"github.com/stretchr/testify/require"
)

func TestResolverReadsOneAtomicVersionedSecretBundle(t *testing.T) {
	var requested bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
		if request.Method != http.MethodGet || request.URL.Path != "/api/v4/secrets/warehouse" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer target-access-token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		wantQuery := url.Values{
			"projectId":              {"project-1"},
			"environment":            {"prod"},
			"secretPath":             {"/leapview/sales"},
			"type":                   {"shared"},
			"expandSecretReferences": {"true"},
		}
		if request.URL.Query().Encode() != wantQuery.Encode() {
			t.Fatalf("query = %q, want %q", request.URL.Query().Encode(), wantQuery.Encode())
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"secret": map[string]any{
			"id": "secret-warehouse", "secretValue": `{"connection_string":"postgres://runtime:source-secret@warehouse/sales"}`,
			"version": 7,
		}})
	}))
	defer server.Close()

	resolver, err := NewResolver(Config{
		BaseURL: server.URL, HTTPClient: server.Client(),
		Authenticator: staticAuthenticator{token: AccessToken{value: "target-access-token", expiresAt: time.Now().Add(time.Hour)}},
		Now:           time.Now,
		SnapshotTTL:   10 * time.Minute,
		MaxBundleSize: 64 << 10,
		AllowedScopes: testAllowedScopes(),
	})
	require.NoError(t, err)
	snapshot, err := resolver.Resolve(context.Background(), connectionbinding.CredentialReference{
		ProjectID: "project-1", Environment: "prod", SecretPath: "/leapview/sales", SecretKey: "warehouse",
	})
	require.NoError(t, err)
	if !requested || snapshot.ProviderVersion() != "secret-warehouse:v7" {
		t.Fatalf("requested=%t provider version=%q", requested, snapshot.ProviderVersion())
	}
	if err := snapshot.Use(func(values map[string]string) error {
		if values["connection_string"] != "postgres://runtime:source-secret@warehouse/sales" || len(values) != 1 {
			t.Fatalf("bundle = %#v", values)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResolverMapsProviderFailuresWithoutLeakingResponseValues(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   error
	}{
		{name: "denied", status: http.StatusForbidden, want: connectionbinding.ErrCredentialDenied},
		{name: "missing", status: http.StatusNotFound, want: connectionbinding.ErrCredentialNotFound},
		{name: "rate limited", status: http.StatusTooManyRequests, want: connectionbinding.ErrCredentialRateLimited},
		{name: "outage", status: http.StatusServiceUnavailable, want: connectionbinding.ErrProviderUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, `{"message":"source-secret-must-not-leak"}`)
			}))
			defer server.Close()
			resolver, err := NewResolver(Config{
				BaseURL: server.URL, HTTPClient: server.Client(),
				Authenticator: staticAuthenticator{token: AccessToken{value: "access", expiresAt: time.Now().Add(time.Hour)}},
				Now:           time.Now, AllowedScopes: testAllowedScopes(),
			})
			require.NoError(t, err)
			_, err = resolver.Resolve(context.Background(), connectionbinding.CredentialReference{
				ProjectID: "project-1", Environment: "prod", SecretPath: "/", SecretKey: "warehouse",
			})
			if !errors.Is(err, test.want) || strings.Contains(err.Error(), "source-secret") {
				t.Fatalf("Resolve() error = %v, want %v without response disclosure", err, test.want)
			}
		})
	}
}

func TestUniversalAuthenticatorCachesAndRefreshesShortLivedAccessTokens(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	var logins int
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/auth/universal-auth/login" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("content type = %q", request.Header.Get("Content-Type"))
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("clientId") != "machine-client" || request.Form.Get("clientSecret") != "bootstrap-secret" {
			t.Fatalf("login form = %#v", request.Form)
		}
		mu.Lock()
		logins++
		current := logins
		mu.Unlock()
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"accessToken": "access-" + string(rune('0'+current)), "expiresIn": 120, "tokenType": "Bearer",
		})
	}))
	defer server.Close()

	authenticator, err := NewUniversalAuthenticator(UniversalAuthConfig{
		BaseURL: server.URL, ClientID: "machine-client", ClientSecret: "bootstrap-secret",
		HTTPClient: server.Client(), Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	first, err := authenticator.AccessToken(context.Background())
	require.NoError(t, err)
	second, err := authenticator.AccessToken(context.Background())
	require.NoError(t, err)
	if first.value != "access-1" || second.value != first.value || logins != 1 {
		t.Fatalf("first=%q second=%q logins=%d", first.value, second.value, logins)
	}
	now = now.Add(91 * time.Second)
	rotated, err := authenticator.AccessToken(context.Background())
	require.NoError(t, err)
	if rotated.value != "access-2" || logins != 2 {
		t.Fatalf("rotated=%q logins=%d", rotated.value, logins)
	}
}

func TestResolverInvalidatesRejectedAccessTokenAndRetriesOnce(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	logins := 0
	reads := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch request.URL.Path {
		case "/api/v1/auth/universal-auth/login":
			logins++
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"accessToken": "access-" + strconv.Itoa(logins), "expiresIn": 300, "tokenType": "Bearer",
			})
		case "/api/v4/secrets/warehouse":
			reads++
			if request.Header.Get("Authorization") == "Bearer access-1" {
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			if request.Header.Get("Authorization") != "Bearer access-2" {
				t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"secret": map[string]any{
				"id": "secret-warehouse", "secretValue": `{"password":"rotated-source-secret"}`, "version": 8,
			}})
		default:
			t.Fatalf("unexpected request path %q", request.URL.Path)
		}
	}))
	defer server.Close()
	authenticator, err := NewUniversalAuthenticator(UniversalAuthConfig{
		BaseURL: server.URL, ClientID: "machine-client", ClientSecret: "bootstrap-secret",
		HTTPClient: server.Client(), Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	resolver, err := NewResolver(Config{
		BaseURL: server.URL, HTTPClient: server.Client(), Authenticator: authenticator,
		Now: func() time.Time { return now }, AllowedScopes: testAllowedScopes(),
	})
	require.NoError(t, err)
	snapshot, err := resolver.Resolve(context.Background(), connectionbinding.CredentialReference{
		ProjectID: "project-1", Environment: "prod", SecretPath: "/leapview/sales", SecretKey: "warehouse",
	})
	require.NoError(t, err)
	defer snapshot.Destroy()
	if snapshot.ProviderVersion() != "secret-warehouse:v8" || logins != 2 || reads != 2 {
		t.Fatalf("version=%q logins=%d reads=%d", snapshot.ProviderVersion(), logins, reads)
	}
}

func TestProductionResolverRejectsInsecureOrConfusedDeputyConfiguration(t *testing.T) {
	for _, baseURL := range []string{"http://infisical.internal", "https://user:password@infisical.example.com", "https://infisical.example.com/base/path"} {
		if _, err := NewResolver(Config{
			BaseURL: baseURL, HTTPClient: http.DefaultClient,
			Authenticator: staticAuthenticator{token: AccessToken{value: "access", expiresAt: time.Now().Add(time.Hour)}},
			Now:           time.Now, AllowedScopes: testAllowedScopes(),
		}); !errors.Is(err, connectionbinding.ErrInvalidBinding) {
			t.Fatalf("NewResolver(%q) error = %v", baseURL, err)
		}
	}
}

func TestResolverRejectsReferencesOutsideOperatorAllowedScopesBeforeAuthentication(t *testing.T) {
	authenticator := &countingAuthenticator{token: AccessToken{value: "access", expiresAt: time.Now().Add(time.Hour)}}
	resolver, err := NewResolver(Config{
		BaseURL: "https://infisical.example.com", HTTPClient: http.DefaultClient, Authenticator: authenticator,
		Now: time.Now, AllowedScopes: []AllowedScope{{
			ProjectID: "project-1", Environment: "prod", SecretPathPrefix: "/leapview/sales",
		}},
	})
	require.NoError(t, err)
	for _, reference := range []connectionbinding.CredentialReference{
		{ProjectID: "project-2", Environment: "prod", SecretPath: "/leapview/sales", SecretKey: "warehouse"},
		{ProjectID: "project-1", Environment: "dev", SecretPath: "/leapview/sales", SecretKey: "warehouse"},
		{ProjectID: "project-1", Environment: "prod", SecretPath: "/leapview/finance", SecretKey: "warehouse"},
	} {
		if _, err := resolver.Resolve(context.Background(), reference); !errors.Is(err, connectionbinding.ErrCredentialDenied) {
			t.Fatalf("Resolve(%#v) error = %v", reference, err)
		}
	}
	if authenticator.calls != 0 {
		t.Fatalf("authenticator calls = %d, want no confused-deputy provider request", authenticator.calls)
	}
}

func TestOIDCAuthenticatorExchangesWorkloadIdentityWithoutPersistingIt(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/auth/oidc-auth/login" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("identityId") != "identity-1" || request.Form.Get("jwt") != "workload-jwt" {
			t.Fatalf("OIDC form = %#v", request.Form)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"accessToken": "oidc-access", "expiresIn": 300, "tokenType": "Bearer",
		})
	}))
	defer server.Close()
	authenticator, err := NewOIDCAuthenticator(OIDCAuthConfig{
		BaseURL: server.URL, IdentityID: "identity-1", IdentityTokens: staticIdentityTokenSource("workload-jwt"),
		HTTPClient: server.Client(), Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	token, err := authenticator.AccessToken(context.Background())
	require.NoError(t, err)
	if token.value != "oidc-access" {
		t.Fatalf("access token = %q", token.value)
	}
	formatted := strings.Join([]string{
		authenticator.String(), authenticator.GoString(), authenticator.config.String(), authenticator.config.GoString(),
	}, " ")
	for _, secret := range []string{"workload-jwt", "oidc-access"} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("formatted OIDC authenticator leaked %q: %s", secret, formatted)
		}
	}
}

func TestUniversalAuthenticatorNeverForwardsBootstrapSecretAcrossRedirects(t *testing.T) {
	var forwarded bool
	destination := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		forwarded = true
	}))
	defer destination.Close()
	redirector := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	authenticator, err := NewUniversalAuthenticator(UniversalAuthConfig{
		BaseURL: redirector.URL, ClientID: "machine-client", ClientSecret: "bootstrap-secret",
		HTTPClient: redirector.Client(), Now: time.Now,
	})
	require.NoError(t, err)
	if _, err := authenticator.AccessToken(context.Background()); !errors.Is(err, connectionbinding.ErrProviderUnavailable) {
		t.Fatalf("redirected login error = %v", err)
	}
	if forwarded {
		t.Fatal("Universal Auth bootstrap secret was forwarded across a redirect")
	}
}

type staticAuthenticator struct {
	token AccessToken
	err   error
}

type countingAuthenticator struct {
	token AccessToken
	calls int
}

func (authenticator *countingAuthenticator) AccessToken(context.Context) (AccessToken, error) {
	authenticator.calls++
	return authenticator.token, nil
}

func testAllowedScopes() []AllowedScope {
	return []AllowedScope{{ProjectID: "project-1", Environment: "prod", SecretPathPrefix: "/"}}
}

func (authenticator staticAuthenticator) AccessToken(context.Context) (AccessToken, error) {
	return authenticator.token, authenticator.err
}

type staticIdentityTokenSource string

func (source staticIdentityTokenSource) IdentityToken(context.Context) (string, error) {
	return string(source), nil
}
