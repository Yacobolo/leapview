package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	apigenapi "github.com/flidai/leapview/internal/app/api/gen"
	projectbundle "github.com/flidai/leapview/internal/project/bundle"
	releasegen "github.com/flidai/leapview/internal/release/api/gen"
	"github.com/flidai/leapview/internal/workspace/api"
	"github.com/stretchr/testify/require"
)

func TestDeployPreparesCompleteProjectBeforeOneAtomicActivation(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	revision := "sha256:" + strings.Repeat("a", 64)
	workspaces := []string{"operations", "sales", "visuals"}
	var mu sync.Mutex
	var sequence []string
	artifactDigests := map[string]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sequence = append(sequence, r.Method+" "+r.URL.Path)
		mu.Unlock()
		workspaceID := workspaceIDFromAPIPath(r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/instance":
			writeCLIJSON(t, w, apigenapi.InstanceResponse{Environment: "prod"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/capabilities":
			writeCLIJSON(t, w, apigenapi.CapabilitiesResponse{ApiVersion: "v1", BuildVersion: "test", Environment: "prod", Authentication: []apigenapi.AuthenticationMode{apigenapi.AuthenticationModeBearer}, QueryFormats: []apigenapi.QueryFormat{apigenapi.QueryFormatApplicationJson}, UploadProtocols: []apigenapi.UploadProtocol{apigenapi.UploadProtocolTus}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/active-asset-graph"):
			writeCLIJSON(t, w, activeGraphResponse(nil, nil))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/leapview-showcase/releases":
			var request releasegen.ReleaseCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, releasegen.ReleaseResponse{Id: "release-1", ProjectId: "leapview-showcase", ProjectDigest: request.ProjectDigest, Status: releasegen.ReleaseStatusDraft, CreatedBy: "test", CreatedAt: "2026-01-01T00:00:00Z", Workspaces: request.Workspaces, Connections: request.Connections})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/artifact"):
			pins, digest := readManagedDataPinsFromUpload(t, r.Body)
			if len(pins) != 1 || pins["olist"] != revision {
				t.Fatalf("%s managed pins = %#v", workspaceID, pins)
			}
			artifactDigests[workspaceID] = digest
			writeCLIJSON(t, w, releasegen.ReleaseArtifactResponse{ReleaseId: "release-1", WorkspaceId: workspaceID, Digest: digest, SizeBytes: 1})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/releases/release-1/finalize"):
			writeCLIJSON(t, w, releasegen.ReleaseResponse{Id: "release-1", ProjectId: "leapview-showcase", ProjectDigest: "ready", Status: releasegen.ReleaseStatusValidating, CreatedBy: "test", CreatedAt: "2026-01-01T00:00:00Z", Workspaces: []releasegen.ReleaseWorkspaceManifest{}, Connections: []releasegen.ReleaseConnectionPin{}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/leapview-showcase/releases/release-1":
			writeCLIJSON(t, w, releasegen.ReleaseResponse{Id: "release-1", ProjectId: "leapview-showcase", ProjectDigest: "ready", Status: releasegen.ReleaseStatusReady, CreatedBy: "test", CreatedAt: "2026-01-01T00:00:00Z", Workspaces: []releasegen.ReleaseWorkspaceManifest{}, Connections: []releasegen.ReleaseConnectionPin{}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/leapview-showcase/deployments":
			writeCLIJSON(t, w, map[string]any{
				"id": "deployment-1", "projectId": "leapview-showcase", "releaseId": "release-1", "environment": "prod", "status": "queued", "createdBy": "test", "createdAt": "2026-01-01T00:00:00Z",
				"targets": []any{}, "connections": []any{},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/leapview-showcase/deployments/deployment-1":
			writeCLIJSON(t, w, map[string]any{
				"id": "deployment-1", "projectId": "leapview-showcase", "releaseId": "release-1", "environment": "prod", "status": "active", "createdBy": "test", "createdAt": "2026-01-01T00:00:00Z",
				"targets": []any{}, "connections": []any{},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runDeploy(context.Background(), deployRequest{
		ProjectPath: projectPath,
		Revisions:   map[string]string{"olist": revision},
		Target:      server.URL,
		Token:       "secret-token",
		AutoApprove: true,
		Out:         &out,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("runDeploy() error = %v", err)
	}
	if strings.Contains(out.String(), "secret-token") || !strings.Contains(out.String(), "deployed leapview-showcase release=release-1 deployment=deployment-1 environment=prod status=active") {
		t.Fatalf("output = %q", out.String())
	}
	assertSequenceContainsInOrder(t, sequence, []string{
		"GET /api/v1/instance",
		"GET /api/v1/capabilities",
		"GET /api/v1/workspaces/operations/active-asset-graph",
		"GET /api/v1/workspaces/sales/active-asset-graph",
		"POST /api/v1/projects/leapview-showcase/releases",
		"PUT /api/v1/projects/leapview-showcase/releases/release-1/workspaces/operations/artifact",
		"POST /api/v1/projects/leapview-showcase/releases/release-1/finalize",
		"GET /api/v1/projects/leapview-showcase/releases/release-1",
		"POST /api/v1/projects/leapview-showcase/deployments",
		"GET /api/v1/projects/leapview-showcase/deployments/deployment-1",
	})
	for _, workspaceID := range workspaces {
		if artifactDigests[workspaceID] == "" {
			t.Fatalf("workspace %s was not prepared", workspaceID)
		}
	}
}

func TestDeployRecoversWhenReleaseAdvancesDuringArtifactReplay(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	revision := "sha256:" + strings.Repeat("a", 64)
	projectDigest := ""
	artifactConflicts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/instance":
			writeCLIJSON(t, w, apigenapi.InstanceResponse{Environment: "prod"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/capabilities":
			writeCLIJSON(t, w, apigenapi.CapabilitiesResponse{
				ApiVersion: "v1", BuildVersion: "test", Environment: "prod",
				Authentication:  []apigenapi.AuthenticationMode{apigenapi.AuthenticationModeBearer},
				QueryFormats:    []apigenapi.QueryFormat{apigenapi.QueryFormatApplicationJson},
				UploadProtocols: []apigenapi.UploadProtocol{apigenapi.UploadProtocolTus},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/active-asset-graph"):
			writeCLIJSON(t, w, activeGraphResponse(nil, nil))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/leapview-showcase/releases":
			var request releasegen.ReleaseCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			projectDigest = request.ProjectDigest
			writeCLIJSON(t, w, releasegen.ReleaseResponse{
				Id: "release-1", ProjectId: "leapview-showcase", ProjectDigest: projectDigest,
				Status: releasegen.ReleaseStatusDraft, CreatedBy: "test", CreatedAt: "2026-01-01T00:00:00Z",
				Workspaces: request.Workspaces, Connections: request.Connections,
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/artifact"):
			artifactConflicts++
			http.Error(w, "release advanced", http.StatusConflict)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/leapview-showcase/releases/release-1":
			writeCLIJSON(t, w, releasegen.ReleaseResponse{
				Id: "release-1", ProjectId: "leapview-showcase", ProjectDigest: projectDigest,
				Status: releasegen.ReleaseStatusReady, CreatedBy: "test", CreatedAt: "2026-01-01T00:00:00Z",
				Workspaces: []releasegen.ReleaseWorkspaceManifest{}, Connections: []releasegen.ReleaseConnectionPin{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/leapview-showcase/deployments":
			writeCLIJSON(t, w, map[string]any{
				"id": "deployment-1", "projectId": "leapview-showcase", "releaseId": "release-1",
				"environment": "prod", "status": "queued", "createdBy": "test",
				"createdAt": "2026-01-01T00:00:00Z", "targets": []any{}, "connections": []any{},
				"approval": map[string]any{
					"id": "approval-1", "projectId": "leapview-showcase",
					"deploymentId": "deployment-1", "environment": "prod",
					"requestDigest": "sha256:plan", "releaseId": "release-1",
					"status": "pending", "requestedBy": "publisher",
					"requestedAt": "2026-01-01T00:00:00Z",
					"expiresAt":   "2026-01-01T00:30:00Z", "revision": 1,
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runDeploy(context.Background(), deployRequest{
		ProjectPath: projectPath,
		Revisions:   map[string]string{"olist": revision},
		Target:      server.URL,
		Token:       "secret-token",
		AutoApprove: true,
		Out:         &out,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("runDeploy() error = %v", err)
	}
	if artifactConflicts != 1 {
		t.Fatalf("artifact conflict attempts = %d, want 1", artifactConflicts)
	}
	if !strings.Contains(out.String(), "published leapview-showcase") ||
		!strings.Contains(out.String(), "approval=approval-1") ||
		!strings.Contains(out.String(), "approval_status=pending") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDeployRejectsIncompleteManagedRevisionSetBeforeNetworkAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid deployment reached server")
	}))
	defer server.Close()

	err := runDeploy(context.Background(), deployRequest{
		ProjectPath: filepath.Join("..", "..", "..", "dashboards", "leapview.yaml"),
		Revisions:   map[string]string{},
		Target:      server.URL,
		Token:       "token",
		AutoApprove: true,
		Out:         &bytes.Buffer{},
		HTTPClient:  server.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "olist") || !strings.Contains(err.Error(), "revision") {
		t.Fatalf("runDeploy() error = %v, want missing olist revision", err)
	}
}

func TestDeployRejectsInheritedWorkspaceTargeting(t *testing.T) {
	command := deployCommand(context.Background(), &rootOptions{workspaceID: "sales"})
	command.SetArgs(nil)
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "project-wide") {
		t.Fatalf("deploy error = %v, want project-wide rejection", err)
	}
}

func assertRequestsBefore(t *testing.T, sequence []string, current string, required []string) {
	t.Helper()
	for _, want := range required {
		found := false
		for _, got := range sequence {
			if got == current {
				break
			}
			if got == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s occurred before %s; sequence=%#v", current, want, sequence)
		}
	}
}

func readManagedDataPinsFromUpload(t *testing.T, body io.Reader) (map[string]string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "artifact.tar.gz")
	file, err := os.Create(path)
	require.NoError(t, err)
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, digest), body); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := projectbundle.ExtractArtifact(path, root); err != nil {
		t.Fatal(err)
	}
	compiled, _, err := projectbundle.LoadCompiledWorkspaceArtifact(root)
	require.NoError(t, err)
	return compiled.ManagedDataRevisions, hex.EncodeToString(digest.Sum(nil))
}

func workspaceIDFromAPIPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index := range parts {
		if parts[index] == "workspaces" && index+1 < len(parts) {
			return parts[index+1]
		}
	}
	return ""
}

func assertSequenceContainsInOrder(t *testing.T, sequence, want []string) {
	t.Helper()
	position := 0
	for _, request := range sequence {
		if position < len(want) && request == want[position] {
			position++
		}
	}
	if position != len(want) {
		t.Fatalf("sequence = %#v, want in order %#v", sequence, want)
	}
}

func activeGraphResponse(_ any, _ any) api.WorkspaceAssetGraphResponse {
	return api.WorkspaceAssetGraphResponse{}
}
