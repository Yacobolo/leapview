package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	accesscli "github.com/flidai/leapview/internal/access/cli"
	"github.com/flidai/leapview/internal/platform/cliapi"
)

func TestTargetEnvironmentDiscoversAndAssertsInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/instance" || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("request = %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"environment":"prod"}`))
	}))
	defer server.Close()
	if got, err := targetEnvironment(context.Background(), server.Client(), server.URL, "token", ""); err != nil || got != "prod" {
		t.Fatalf("environment = %q, %v", got, err)
	}
	if _, err := targetEnvironment(context.Background(), server.Client(), server.URL, "token", "staging"); err == nil {
		t.Fatal("mismatched assertion succeeded")
	}
}

func TestCapabilityAPIClientRejectsLegacyPlaintextTokenConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.json")
	t.Setenv("LEAPVIEW_CLI_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{"version":1,"targets":{"local":{"origin":"http://localhost:8080","instanceId":"lvinst_local","projectId":"project","credentialAccount":"account","token":"secret"}}}`), 0o600); err != nil {
		t.Fatalf("write client config: %v", err)
	}
	_, err := (capabilityAPIClient{}).Resolve(context.Background(), cliapi.Credentials{Target: "local"})
	if err == nil || !strings.Contains(err.Error(), "secret-bearing") {
		t.Fatalf("Resolve error = %v, want plaintext credential rejection", err)
	}
}

type fakeAuthoringResolver struct {
	name string
}

func (resolver *fakeAuthoringResolver) Resolve(_ context.Context, name string) (accesscli.ResolvedCredential, error) {
	resolver.name = name
	return accesscli.ResolvedCredential{
		Profile:     cliapi.TargetProfile{Origin: "https://canonical.example.com"},
		AccessToken: "short-lived",
	}, nil
}

func TestCapabilityAPIClientResolvesAuthoringProfile(t *testing.T) {
	resolver := &fakeAuthoringResolver{}
	credentials, err := (capabilityAPIClient{authoring: resolver}).Resolve(context.Background(), cliapi.Credentials{Target: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.name != "prod" || credentials.Target != "https://canonical.example.com" || credentials.Token != "short-lived" {
		t.Fatalf("resolver name=%q credentials=%+v", resolver.name, credentials)
	}
}

func TestCapabilityAPIClientExchangesEphemeralWorkloadIdentity(t *testing.T) {
	t.Setenv("LEAPVIEW_API_TOKEN", "")
	t.Setenv("LEAPVIEW_WORKLOAD_CLIENT_ID", "sp-ci")
	t.Setenv("LEAPVIEW_WORKLOAD_CLIENT_SECRET", "service-secret")
	t.Setenv("LEAPVIEW_WORKLOAD_PROJECT", "analytics")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/instance":
			_, _ = w.Write([]byte(`{"id":"lvinst_prod","canonicalOrigin":"` + "http://" + r.Host + `","environment":"production"}`))
		case "/api/v1/access/workload-token":
			if r.Header.Get("Authorization") != "" {
				t.Fatalf("workload exchange used authorization header %q", r.Header.Get("Authorization"))
			}
			var request struct {
				Scope struct {
					Privileges []string `json:"privileges"`
				} `json:"scope"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if strings.Join(request.Scope.Privileges, ",") != "USE_WORKSPACE,VIEW_ITEM,AUTHOR_PROJECT,PUBLISH_RELEASE,REQUEST_DEPLOYMENT" {
				t.Fatalf("workload privileges = %v", request.Scope.Privileges)
			}
			_, _ = w.Write([]byte(`{
				"accessToken":"ephemeral-access","tokenType":"Bearer","expiresIn":900,
				"session":{"id":"session-1","kind":"workload","clientId":"sp-ci","targetId":"lvinst_prod",
				"projectId":"analytics","privileges":["USE_WORKSPACE","VIEW_ITEM","AUTHOR_PROJECT","PUBLISH_RELEASE","REQUEST_DEPLOYMENT"],
				"createdAt":"2026-07-29T12:00:00Z","expiresAt":"2026-07-29T12:15:00Z"}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	credentials, err := (capabilityAPIClient{httpClient: server.Client()}).Resolve(context.Background(), cliapi.Credentials{Target: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Target != server.URL || credentials.Token != "ephemeral-access" {
		t.Fatalf("credentials = %+v", credentials)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %#o, want %#o", path, got, want)
	}
}
