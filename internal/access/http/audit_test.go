package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/flidai/leapview/internal/access"
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
