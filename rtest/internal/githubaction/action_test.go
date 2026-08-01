package githubaction_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupActionHasSecureProjectAwareContract(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "action", "setup-rtest", "action.yml"))
	for _, want := range []string{
		"using: composite",
		"service-url:",
		"project:",
		"image:",
		"ca-certificate:",
		"oidc-audience:",
		"backend: \"service\"",
		"go build -trimpath",
		"chmod 0600",
		"RTEST_CONFIG",
		"$GITHUB_PATH",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("action.yml missing %q", want)
		}
	}
}

func TestPOCWorkflowIsManualOIDCAndRepositoryScoped(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "..", ".github", "workflows", "rtest-poc.yml"))
	for _, want := range []string{
		"workflow_dispatch:",
		"id-token: write",
		"github.repository == 'flidai/leapview'",
		"environment: rtest-poc",
		"uses: ./rtest/action/setup-rtest",
		"service-url: ${{ vars.RTEST_SERVICE_URL }}",
		"project: poc",
		"rtest doctor",
		"rtest exec -- go test -count=1 -v ./...",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("rtest-poc.yml missing %q", want)
		}
	}
	if strings.Contains(data, "pull_request:") {
		t.Fatal("POC workflow must not run automatically for pull requests")
	}
	for _, forbidden := range []string{"tailscale/github-action", "ssh-private-key", "RTEST_TOKEN"} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("POC workflow still contains legacy authentication %q", forbidden)
		}
	}
}

func TestMainCIUsesOIDCOnlyForTrustedSameRepositoryPullRequests(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "..", ".github", "workflows", "ci.yml"))
	for _, want := range []string{
		"rtest-oidc-e2e:",
		"github.event.pull_request.head.repo.full_name == github.repository",
		"github.actor != 'dependabot[bot]'",
		"environment: rtest-poc",
		"id-token: write",
		"uses: ./rtest/action/setup-rtest",
		"rtest exec -- go test -count=1 -v ./...",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("ci.yml missing rtest OIDC gate %q", want)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
