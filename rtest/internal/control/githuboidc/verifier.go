package githuboidc

import (
	"context"
	"errors"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/flidai/leapview/rtest/internal/control"
)

const Issuer = "https://token.actions.githubusercontent.com"

type Verifier struct {
	verifier *oidc.IDTokenVerifier
}

func New(ctx context.Context, issuer, audience string) (*Verifier, error) {
	if issuer == "" {
		issuer = Issuer
	}
	if audience == "" {
		return nil, errors.New("GitHub OIDC audience is required")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &Verifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

func (v *Verifier) Verify(ctx context.Context, raw string) (control.GitHubClaims, error) {
	if strings.TrimSpace(raw) == "" {
		return control.GitHubClaims{}, errors.New("GitHub OIDC token is required")
	}
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return control.GitHubClaims{}, err
	}
	var rawClaims struct {
		RepositoryOwnerID string `json:"repository_owner_id"`
		RepositoryID      string `json:"repository_id"`
		Repository        string `json:"repository"`
		WorkflowRef       string `json:"workflow_ref"`
		JobWorkflowRef    string `json:"job_workflow_ref"`
		Ref               string `json:"ref"`
		Environment       string `json:"environment"`
		EventName         string `json:"event_name"`
	}
	if err := token.Claims(&rawClaims); err != nil {
		return control.GitHubClaims{}, err
	}
	workflowRef := rawClaims.WorkflowRef
	if workflowRef == "" {
		workflowRef = rawClaims.JobWorkflowRef
	}
	if token.Subject == "" || rawClaims.RepositoryOwnerID == "" || rawClaims.RepositoryID == "" || workflowRef == "" || rawClaims.Ref == "" || rawClaims.EventName == "" {
		return control.GitHubClaims{}, errors.New("GitHub OIDC token is missing required immutable identity or policy claims")
	}
	return control.GitHubClaims{
		Subject: token.Subject, RepositoryOwnerID: rawClaims.RepositoryOwnerID, RepositoryID: rawClaims.RepositoryID,
		Repository: rawClaims.Repository, WorkflowRef: workflowRef, Ref: rawClaims.Ref,
		Environment: rawClaims.Environment, EventName: rawClaims.EventName, ExpiresAt: token.Expiry,
	}, nil
}

var _ interface {
	Verify(context.Context, string) (control.GitHubClaims, error)
} = (*Verifier)(nil)
