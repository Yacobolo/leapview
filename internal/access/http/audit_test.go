package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access"
	accesssqlite "github.com/flidai/leapview/internal/access/sqlite"
	"github.com/flidai/leapview/internal/platform"
)

type auditedMutationRepository struct {
	access.Repository
	called bool
}

func (r *auditedMutationRepository) RunAuditedMutation(ctx context.Context, mutation func(access.Repository) (access.AuditEventInput, error)) error {
	r.called = true
	_, err := mutation(r.Repository)
	return err
}

func TestRunAuditedMutationUsesRepositoryTransaction(t *testing.T) {
	repo := &auditedMutationRepository{}
	request := httptest.NewRequest(stdhttp.MethodPost, "/", nil)
	mutationCalled := false

	err := runAuditedMutation(request, repo, func(access.Repository) (access.AuditEventInput, error) {
		mutationCalled = true
		return access.AuditEventInput{Action: "grant.created"}, nil
	})
	if err != nil {
		t.Fatalf("run audited mutation: %v", err)
	}
	if !repo.called || !mutationCalled {
		t.Fatalf("transaction called = %v, mutation called = %v", repo.called, mutationCalled)
	}
}

func TestCreatePrincipalAuditsDuplicateRejectionSeparately(t *testing.T) {
	ctx := t.Context()
	store, err := platform.Open(ctx, filepath.Join(t.TempDir(), "access.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := accesssqlite.NewRepository(store.SQLDB())
	handler := Handler{Repository: func() (access.Repository, error) { return repo, nil }}

	request := func(displayName string) *stdhttp.Request {
		r := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/principals", strings.NewReader(
			`{"email":"duplicate@example.com","displayName":"`+displayName+`"}`,
		))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	first := httptest.NewRecorder()
	handler.CreatePrincipal(first, request("Original"))
	if first.Code != stdhttp.StatusCreated {
		t.Fatalf("first status = %d, body=%s", first.Code, first.Body.String())
	}
	duplicate := httptest.NewRecorder()
	handler.CreatePrincipal(duplicate, request("Replacement"))
	if duplicate.Code != stdhttp.StatusConflict {
		t.Fatalf("duplicate status = %d, body=%s", duplicate.Code, duplicate.Body.String())
	}

	created, err := repo.ListAuditEvents(ctx, access.AuditEventFilter{Action: "principal.local_user.created"})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := repo.ListAuditEvents(ctx, access.AuditEventFilter{Action: "principal.local_user.create_rejected"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || len(rejected) != 1 || rejected[0].Status != "conflict" {
		t.Fatalf("created/rejected audit events = %d/%d (%+v)", len(created), len(rejected), rejected)
	}
}

func TestDashboardPublicationSubjectsAreLimitedToDataPolicies(t *testing.T) {
	if knownGrantSubjectType(access.SubjectDashboardPublication) {
		t.Fatal("dashboard publication subject was accepted for an RBAC grant")
	}
	if !knownDataPolicySubjectType(access.SubjectDashboardPublication) {
		t.Fatal("dashboard publication subject was rejected for a data policy")
	}
}

func TestDashboardPublicationPrincipalsRejectGenericIdentityMutations(t *testing.T) {
	if principalKindAllowsGenericMutation(access.PrincipalKindDashboardPublication) {
		t.Fatal("dashboard publication principal accepted a generic identity mutation")
	}
	if !principalKindAllowsGenericMutation(access.PrincipalKindUser) {
		t.Fatal("user principal rejected a generic identity mutation")
	}
}

func TestProjectEnvironmentGrantAuditIsPlatformScoped(t *testing.T) {
	request := httptest.NewRequest(stdhttp.MethodPost, "/", nil)
	grant := access.Grant{
		ID:          "grant_reviewer",
		WorkspaceID: "finance",
		ObjectType:  access.SecurableProjectEnvironment,
		ObjectID:    "production",
		SubjectType: access.SubjectPrincipal,
		SubjectID:   "reviewer",
		Privilege:   access.PrivilegeApproveDeployment,
	}

	input := grantAuditInput(request, "grant.created", "admin", grant)
	if input.WorkspaceID != "" {
		t.Fatalf("audit workspace = %q, want platform scope", input.WorkspaceID)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(input.MetadataJSON), &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if metadata["projectId"] != "finance" || metadata["environment"] != "production" {
		t.Fatalf("audit metadata = %#v, want project and environment", metadata)
	}
}

func TestWorkspaceGrantAuditRetainsWorkspaceScope(t *testing.T) {
	request := httptest.NewRequest(stdhttp.MethodPost, "/", nil)
	grant := access.Grant{
		ID:          "grant_dashboard",
		WorkspaceID: "sales",
		ObjectType:  access.SecurableDashboard,
		ObjectID:    "executive",
		SubjectType: access.SubjectPrincipal,
		SubjectID:   "viewer",
		Privilege:   access.PrivilegeViewItem,
	}

	input := grantAuditInput(request, "grant.created", "admin", grant)
	if input.WorkspaceID != "sales" {
		t.Fatalf("audit workspace = %q, want sales", input.WorkspaceID)
	}
}
