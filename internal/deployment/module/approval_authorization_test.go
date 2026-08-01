package module

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/deployment"
	"github.com/flidai/leapview/internal/deployment/apiadapter"
	"github.com/stretchr/testify/require"
)

func TestProtectedActivationReauthorizesApprovalAndActivationCredentials(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	service := approvedService(t, now)
	row := apiadapter.Deployment{
		ID: "deployment_1", Project: "finance", Environment: "prod",
		RequestDigest: "sha256:plan",
	}
	activator := deployment.ApprovalActor{
		PrincipalID: "activator", CredentialClass: deployment.CredentialClassWorkload,
		CredentialID: "activate_session", CredentialExpiresAt: now.Add(time.Hour),
	}

	var approvalChecks, activationChecks int
	module := &Module{
		approvals: service,
		authorizeApproval: func(_ context.Context, actor deployment.ApprovalActor, project, environment string) error {
			approvalChecks++
			if actor.PrincipalID != "reviewer" ||
				actor.CredentialID != "review_session" ||
				!actor.CredentialExpiresAt.Equal(now.Add(time.Hour)) ||
				project != "finance" || environment != "prod" {
				t.Fatalf("approval evidence = %#v project=%q environment=%q", actor, project, environment)
			}
			return nil
		},
		authorizeActivation: func(_ context.Context, actor deployment.ApprovalActor, project, environment string) error {
			activationChecks++
			if actor != activator || project != "finance" || environment != "prod" {
				t.Fatalf("activation evidence = %#v project=%q environment=%q", actor, project, environment)
			}
			return nil
		},
	}

	if _, err := module.authorizeApprovedActivation(
		t.Context(),
		row,
		"release_1",
		activator,
	); err != nil {
		t.Fatal(err)
	}
	if approvalChecks != 1 || activationChecks != 1 {
		t.Fatalf("credential checks = approval:%d activation:%d", approvalChecks, activationChecks)
	}
}

func TestProtectedActivationFailsClosedWhenApprovalCredentialIsRevoked(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	service := approvedService(t, now)
	activationChecked := false
	module := &Module{
		approvals: service,
		authorizeApproval: func(
			context.Context,
			deployment.ApprovalActor,
			string,
			string,
		) error {
			return ErrApprovalForbidden
		},
		authorizeActivation: func(
			context.Context,
			deployment.ApprovalActor,
			string,
			string,
		) error {
			activationChecked = true
			return nil
		},
	}
	_, err := module.authorizeApprovedActivation(
		t.Context(),
		apiadapter.Deployment{
			ID: "deployment_1", Project: "finance", Environment: "prod",
			RequestDigest: "sha256:plan",
		},
		"release_1",
		deployment.ApprovalActor{
			PrincipalID: "activator", CredentialClass: deployment.CredentialClassHuman,
			CredentialID: "activate_session", CredentialExpiresAt: now.Add(time.Hour),
		},
	)
	if !errors.Is(err, ErrApprovalForbidden) {
		t.Fatalf("authorizeApprovedActivation() error = %v", err)
	}
	if activationChecked {
		t.Fatal("activation credential was evaluated after approval evidence failed")
	}
}

func approvedService(t *testing.T, now time.Time) *deployment.ApprovalService {
	t.Helper()
	repository := &approvalRepositoryStub{}
	sequence := 0
	service, err := deployment.NewApprovalService(
		repository,
		deployment.ApprovalServiceConfig{
			Lifetime: 30 * time.Minute,
			Now:      func() time.Time { return now },
			NewID: func() (string, error) {
				sequence++
				return "approval_1", nil
			},
		},
	)
	require.NoError(t, err)
	requested, err := service.Request(t.Context(), deployment.ApprovalRequest{
		ProjectID: "finance", DeploymentID: "deployment_1",
		Environment: "prod", RequestDigest: "sha256:plan", ReleaseID: "release_1",
		RequestedBy: deployment.ApprovalActor{
			PrincipalID: "publisher", CredentialClass: deployment.CredentialClassWorkload,
			CredentialID: "publish_session", CredentialExpiresAt: now.Add(time.Hour),
		},
	})
	require.NoError(t, err)
	if _, err := service.Approve(t.Context(), deployment.ApprovalTransition{
		ProjectID: "finance", DeploymentID: "deployment_1",
		ApprovalID: requested.ID, ExpectedRevision: requested.Revision,
		Actor: deployment.ApprovalActor{
			PrincipalID: "reviewer", CredentialClass: deployment.CredentialClassHuman,
			CredentialID: "review_session", CredentialExpiresAt: now.Add(time.Hour),
		},
	}); err != nil {
		t.Fatal(err)
	}
	return service
}

type approvalRepositoryStub struct {
	current deployment.Approval
}

func (repository *approvalRepositoryStub) CreateApproval(
	_ context.Context,
	approval deployment.Approval,
) (deployment.Approval, error) {
	repository.current = approval
	return approval, nil
}

func (repository *approvalRepositoryStub) ApprovalByDeployment(
	_ context.Context,
	deploymentID string,
) (deployment.Approval, error) {
	if repository.current.DeploymentID != deploymentID {
		return deployment.Approval{}, deployment.ErrApprovalNotFound
	}
	return repository.current, nil
}

func (repository *approvalRepositoryStub) SaveApproval(
	_ context.Context,
	approval deployment.Approval,
	expectedRevision int64,
) (deployment.Approval, error) {
	if repository.current.Revision != expectedRevision {
		return deployment.Approval{}, deployment.ErrApprovalConflict
	}
	repository.current = approval
	return approval, nil
}
