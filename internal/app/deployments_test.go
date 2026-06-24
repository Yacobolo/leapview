package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	accesssqlite "github.com/Yacobolo/libredash/internal/access/sqlite"
	"github.com/Yacobolo/libredash/internal/agentapp"
	agentappsqlite "github.com/Yacobolo/libredash/internal/agentapp/sqlite"
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/deployment"
	deploymentfs "github.com/Yacobolo/libredash/internal/deployment/filesystem"
	deploymentsqlite "github.com/Yacobolo/libredash/internal/deployment/sqlite"
	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
	workspacesqlite "github.com/Yacobolo/libredash/internal/workspace/sqlite"
	"github.com/gorilla/csrf"
)

type fakeReloader struct {
	prepareCalls int
	commitCalls  int
	prepareErr   error
}

func (r *fakeReloader) Reload(context.Context) error {
	r.prepareCalls++
	r.commitCalls++
	return nil
}

func (r *fakeReloader) PrepareDeployment(context.Context, string) (deployment.PreparedRuntime, error) {
	r.prepareCalls++
	if r.prepareErr != nil {
		return nil, r.prepareErr
	}
	return fakePreparedRuntime{}, nil
}

func (r *fakeReloader) CommitPrepared(deployment.PreparedRuntime) error {
	r.commitCalls++
	return nil
}

type fakePreparedRuntime struct{}

func (fakePreparedRuntime) Close() error { return nil }

func TestDeploymentAPIRequiresAuthentication(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeploymentAPIRejectsBrowserPostWithoutCSRF(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/deployments", bytes.NewBufferString(`{"workspaceId":"test"}`))
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRFMiddlewareAllowsBrowserPostWithToken(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	handler := auth.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(csrf.Token(r)))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	getReq := httptest.NewRequest(http.MethodGet, "/form", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/form", nil)
	postReq.Header.Set("X-CSRF-Token", getRec.Body.String())
	postReq.Header.Set("Origin", "http://example.com")
	for _, cookie := range getRec.Result().Cookies() {
		postReq.AddCookie(cookie)
	}
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusNoContent {
		t.Fatalf("POST status = %d, want %d, body=%s", postRec.Code, http.StatusNoContent, postRec.Body.String())
	}
}

func TestCSRFMiddlewareAllowsPlainHTTPPostWithToken(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true, CookieSecure: false})
	handler := auth.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(csrf.Token(r)))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	getReq := httptest.NewRequest(http.MethodGet, "http://localhost:8120/form", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}

	postReq := httptest.NewRequest(http.MethodPost, "http://localhost:8120/form", nil)
	postReq.Header.Set("X-CSRF-Token", getRec.Body.String())
	postReq.Header.Set("Referer", "http://localhost:8120/form")
	for _, cookie := range getRec.Result().Cookies() {
		postReq.AddCookie(cookie)
	}
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusNoContent {
		t.Fatalf("POST status = %d, want %d, body=%s", postRec.Code, http.StatusNoContent, postRec.Body.String())
	}
}

func TestSessionCookieUsesConfiguredSecureFlag(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true, CookieSecure: true})
	cookie := auth.sessionCookie("token", time.Now().Add(time.Hour))
	if !cookie.Secure {
		t.Fatal("session cookie Secure = false, want true")
	}
}

func TestDeploymentAPIRejectsViewer(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	principal := testPrincipal(t, ctx, store, "viewer@example.com", "Viewer", "viewer")
	token := testAPIToken(t, ctx, store, principal.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/deployments", bytes.NewBufferString(`{"workspaceId":"test"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestDeploymentAPIValidatesAndActivatesBundle(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	reloader := &fakeReloader{}
	artifactDir := t.TempDir()
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, Reloader: reloader, ArtifactDir: artifactDir, DefaultWorkspaceID: "test"})

	createReq := httptest.NewRequest(http.MethodPost, "/api/deployments", bytes.NewBufferString(`{"workspaceId":"test","title":"Test"}`))
	createReq.Header.Set("Authorization", "Bearer dev")
	createReq.Header.Set("Accept", "application/json")
	createRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created api.DeploymentResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	var bundle bytes.Buffer
	if _, _, err := deploymentfs.PackCatalog(filepath.Join("..", "..", "dashboards", "catalog.yaml"), &bundle); err != nil {
		t.Fatalf("pack catalog: %v", err)
	}
	uploadReq := httptest.NewRequest(http.MethodPut, "/api/deployments/"+created.ID+"/artifact", bytes.NewReader(bundle.Bytes()))
	uploadReq.Header.Set("Authorization", "Bearer dev")
	uploadReq.Header.Set("Accept", "application/json")
	uploadRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	validateReq := httptest.NewRequest(http.MethodPost, "/api/deployments/"+created.ID+"/validate", nil)
	validateReq.Header.Set("Authorization", "Bearer dev")
	validateReq.Header.Set("Accept", "application/json")
	validateRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(validateRec, validateReq)
	if validateRec.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateRec.Code, validateRec.Body.String())
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/api/deployments/"+created.ID+"/activate", nil)
	activateReq.Header.Set("Authorization", "Bearer dev")
	activateReq.Header.Set("Accept", "application/json")
	activateRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(activateRec, activateReq)
	if activateRec.Code != http.StatusOK {
		t.Fatalf("activate status = %d body=%s", activateRec.Code, activateRec.Body.String())
	}
	if reloader.prepareCalls != 1 {
		t.Fatalf("prepare calls = %d, want 1", reloader.prepareCalls)
	}
	if reloader.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", reloader.commitCalls)
	}
}

func TestDeploymentActivationPrepareFailureLeavesDeploymentInactive(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	ctx := context.Background()
	deploymentRepo := deploymentsqlite.NewRepository(store.SQLDB())
	created, err := deploymentRepo.Create(ctx, deployment.CreateInput{WorkspaceID: "test", CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	if _, err := deploymentRepo.SaveValidated(ctx, created.ID, deployment.Validation{Digest: "digest", ManifestJSON: "{}"}, zeroArtifact(created.ID, "test")); err != nil {
		t.Fatalf("validate deployment: %v", err)
	}
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	reloader := &fakeReloader{prepareErr: errors.New("runtime load failed")}
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, Reloader: reloader, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/deployments/"+string(created.ID)+"/activate", nil)
	req.Header.Set("Authorization", "Bearer dev")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	after, err := deploymentRepo.ByID(ctx, deployment.ID(created.ID))
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if after.Status != deployment.StatusValidated {
		t.Fatalf("status = %q, want validated", after.Status)
	}
	if reloader.commitCalls != 0 {
		t.Fatalf("commit calls = %d, want 0", reloader.commitCalls)
	}
}

func TestWorkspaceAssetAPIListsActiveDeploymentAssets(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	seedActiveDeployment(t, store, "test")
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/test/assets?type=connection", nil)
	req.Header.Set("Authorization", "Bearer dev")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"connection"`)) {
		t.Fatalf("connection asset missing:\n%s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"auth"`)) {
		t.Fatalf("connection API leaked auth content:\n%s", rec.Body.String())
	}
}

func TestWorkspacePageDefaultsToTopLevelAssets(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	seedActiveDeployment(t, store, "test")
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Executive Sales Dashboard", "Olist Commerce"} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace page missing top-level asset %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"olist_orders_dataset.csv", "orders_enriched", "review_score"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("workspace page rendered low-level asset %q:\n%s", notWant, body)
		}
	}
}

func TestWorkspaceAssetSearchStaysWorkspaceFacing(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	seedActiveDeployment(t, store, "test")
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test?q=orders_enriched", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, notWant := range []string{"Cache table", "Dataset", `>orders_enriched<`} {
		if strings.Contains(rec.Body.String(), notWant) {
			t.Fatalf("workspace search rendered internal asset vocabulary %q:\n%s", notWant, rec.Body.String())
		}
	}
}

func TestWorkspaceConnectionFilterRedirectsToGlobalConnections(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	seedActiveDeployment(t, store, "test")
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test?type=connection&q=olist", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/connections?q=olist" {
		t.Fatalf("Location = %q, want /connections?q=olist", got)
	}
}

func TestConnectionsPageRendersGlobalConnectionSurface(t *testing.T) {
	t.Setenv("LIBREDASH_DEV_AUTH_BYPASS", "1")
	store := testStore(t)
	seedActiveDeployment(t, store, "test")
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/connections?q=olist", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Connections", "Global", "data-connection-toolbar", "local connection"} {
		if !strings.Contains(body, want) {
			t.Fatalf("connections page missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `data-workspace-asset-toolbar`) {
		t.Fatalf("connections page rendered workspace asset toolbar:\n%s", body)
	}
}

func TestWorkspacePermissionsRejectViewer(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	principal := testPrincipal(t, ctx, store, "viewer@example.com", "Viewer", "viewer")
	token := testAPIToken(t, ctx, store, principal.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestWorkspaceRoleBindingAPIUpsertsPrincipalRole(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", "owner")
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/test/role-bindings", bytes.NewBufferString(`{"email":"analyst@example.com","displayName":"Analyst","role":"viewer"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d body=%s", rec.Code, rec.Body.String())
	}
	updateReq := httptest.NewRequest(http.MethodPost, "/api/workspaces/test/role-bindings", bytes.NewBufferString(`{"email":"analyst@example.com","displayName":"Analyst","role":"editor"}`))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Accept", "application/json")
	updateRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/test/role-bindings", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listReq.Header.Set("Accept", "application/json")
	listRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	body := listRec.Body.String()
	if !strings.Contains(body, `"email":"analyst@example.com"`) || !strings.Contains(body, `"role":"editor"`) {
		t.Fatalf("role binding missing:\n%s", body)
	}
	if strings.Contains(body, `"role":"viewer"`) {
		t.Fatalf("role binding was duplicated instead of replaced:\n%s", body)
	}
}

func TestWorkspaceAccessCommandUpsertsAndPatchesSignals(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", "owner")
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	signals := `{"workspaceAccess":{"command":{"email":"analyst@example.com","role":"viewer"}}}`
	req := httptest.NewRequest(http.MethodPost, "/workspaces/test/access/upsert", bytes.NewBufferString(signals))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"event: datastar-patch-signals", "workspaceAccess", "analyst@example.com", "Access updated."} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace access upsert did not patch %q:\n%s", want, body)
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/test/role-bindings", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listReq.Header.Set("Accept", "application/json")
	listRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"email":"analyst@example.com"`) {
		t.Fatalf("role binding missing after command:\n%s", listRec.Body.String())
	}

	removeSignals := `{"workspaceAccess":{"command":{"principalId":"` + access.PrincipalIDForEmail("analyst@example.com") + `"}}}`
	removeReq := httptest.NewRequest(http.MethodPost, "/workspaces/test/access/remove", bytes.NewBufferString(removeSignals))
	removeReq.Header.Set("Authorization", "Bearer "+token)
	removeRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(removeRec, removeReq)
	if removeRec.Code != http.StatusOK {
		t.Fatalf("remove status = %d body=%s", removeRec.Code, removeRec.Body.String())
	}
	if !strings.Contains(removeRec.Body.String(), "Access removed.") {
		t.Fatalf("workspace access remove did not patch success:\n%s", removeRec.Body.String())
	}

	removedListReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/test/role-bindings", nil)
	removedListReq.Header.Set("Authorization", "Bearer "+token)
	removedListReq.Header.Set("Accept", "application/json")
	removedListRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(removedListRec, removedListReq)
	if strings.Contains(removedListRec.Body.String(), `"email":"analyst@example.com"`) {
		t.Fatalf("role binding remained after remove command:\n%s", removedListRec.Body.String())
	}
}

func TestWorkspaceAccessCommandRejectsViewer(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	viewer := testPrincipal(t, ctx, store, "viewer@example.com", "Viewer", "viewer")
	token := testAPIToken(t, ctx, store, viewer.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	signals := `{"workspaceAccess":{"command":{"email":"analyst@example.com","role":"viewer"}}}`
	req := httptest.NewRequest(http.MethodPost, "/workspaces/test/access/upsert", bytes.NewBufferString(signals))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestWorkspaceAccessCommandPatchesInvalidInput(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", "owner")
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, ArtifactDir: t.TempDir(), DefaultWorkspaceID: "test"})

	signals := `{"workspaceAccess":{"command":{"email":"","role":"viewer"}}}`
	req := httptest.NewRequest(http.MethodPost, "/workspaces/test/access/upsert", bytes.NewBufferString(signals))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "email is required") {
		t.Fatalf("invalid access command did not patch validation error:\n%s", body)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/test/role-bindings", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listReq.Header.Set("Accept", "application/json")
	listRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(listRec, listReq)
	if strings.Contains(listRec.Body.String(), "analyst@example.com") {
		t.Fatalf("invalid command changed bindings:\n%s", listRec.Body.String())
	}
}

func testStore(t *testing.T) *platform.Store {
	t.Helper()
	store, err := platform.Open(context.Background(), filepath.Join(t.TempDir(), "libredash.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	workspaceRepo := workspacesqlite.NewRepository(store.SQLDB())
	if err := workspaceRepo.Ensure(context.Background(), workspace.EnsureInput{ID: workspace.WorkspaceID("test"), Title: "Test"}); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	return store
}

func testAccessRepository(store *platform.Store) access.Repository {
	return accesssqlite.NewRepository(store.SQLDB())
}

func testAgentRepository(store *platform.Store) agentapp.Repository {
	return agentappsqlite.NewRepository(store.SQLDB())
}

func testPrincipal(t *testing.T, ctx context.Context, store *platform.Store, email, displayName, role string) access.Principal {
	t.Helper()
	repo := testAccessRepository(store)
	if role != "" {
		principal, err := repo.SetPrincipalRole(ctx, access.PrincipalRoleInput{WorkspaceID: "test", Email: email, DisplayName: displayName, Role: role})
		if err != nil {
			t.Fatalf("bind role: %v", err)
		}
		return principal
	}
	principal, err := repo.UpsertPrincipal(ctx, access.PrincipalInput{Email: email, DisplayName: displayName})
	if err != nil {
		t.Fatalf("upsert principal: %v", err)
	}
	return principal
}

func testAPIToken(t *testing.T, ctx context.Context, store *platform.Store, principalID, name string) string {
	t.Helper()
	token, err := testAccessRepository(store).CreateAPIToken(ctx, principalID, name)
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}
	return token
}

func seedActiveDeployment(t *testing.T, store *platform.Store, workspaceID string) {
	t.Helper()
	ctx := context.Background()
	deploymentRepo := deploymentsqlite.NewRepository(store.SQLDB())
	created, err := deploymentRepo.Create(ctx, deployment.CreateInput{WorkspaceID: deployment.WorkspaceID(workspaceID), CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	workspaceDef, err := workspacecompiler.CompileDefinition(filepath.Join("..", "..", "dashboards", "catalog.yaml"))
	if err != nil {
		t.Fatalf("compile workspace definition: %v", err)
	}
	graph, err := workspacecompiler.ExtractLineage(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(created.ID), workspaceDef)
	if err != nil {
		t.Fatalf("extract assets: %v", err)
	}
	validation := deployment.Validation{
		Digest:       "digest-" + string(created.ID),
		ManifestJSON: "{}",
		Assets:       deploymentAssets(graph.Assets),
		Edges:        deploymentEdges(graph.Edges),
	}
	if _, err := deploymentRepo.SaveValidated(ctx, created.ID, validation, zeroArtifact(created.ID, workspaceID)); err != nil {
		t.Fatalf("validate deployment: %v", err)
	}
	if _, err := deploymentRepo.Activate(ctx, deployment.WorkspaceID(workspaceID), created.ID); err != nil {
		t.Fatalf("activate deployment: %v", err)
	}
}

func zeroArtifact(deploymentID deployment.ID, workspaceID string) deployment.Artifact {
	return deployment.Artifact{
		ID:           "artifact_" + string(deploymentID),
		DeploymentID: deploymentID,
		WorkspaceID:  deployment.WorkspaceID(workspaceID),
		Digest:       "digest",
		Format:       "tar.gz",
		Path:         "artifact.tar.gz",
		ManifestJSON: "{}",
	}
}

func deploymentAssets(rows []workspace.Asset) []deployment.Asset {
	assets := make([]deployment.Asset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, deployment.Asset{
			ID:           string(row.ID),
			WorkspaceID:  deployment.WorkspaceID(row.WorkspaceID),
			DeploymentID: deployment.ID(row.DeploymentID),
			Type:         string(row.Type),
			Key:          row.Key,
			ParentID:     string(row.ParentID),
			Title:        row.Title,
			Description:  row.Description,
			ContentJSON:  row.ContentJSON,
			ContentHash:  row.ContentHash,
		})
	}
	return assets
}

func deploymentEdges(rows []workspace.AssetEdge) []deployment.AssetEdge {
	edges := make([]deployment.AssetEdge, 0, len(rows))
	for _, row := range rows {
		edges = append(edges, deployment.AssetEdge{
			ID:           string(row.ID),
			WorkspaceID:  deployment.WorkspaceID(row.WorkspaceID),
			DeploymentID: deployment.ID(row.DeploymentID),
			FromAssetID:  string(row.FromAssetID),
			ToAssetID:    string(row.ToAssetID),
			Type:         string(row.Type),
		})
	}
	return edges
}
