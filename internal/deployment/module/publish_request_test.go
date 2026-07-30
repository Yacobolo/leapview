package module

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/deployment"
	"github.com/flidai/leapview/internal/deployment/apiadapter"
	deploymenthttp "github.com/flidai/leapview/internal/deployment/http"
	"github.com/flidai/leapview/internal/platform/transaction"
	"github.com/flidai/leapview/internal/release"
)

func TestPublishEvidenceAcceptsExactTargetRelease(t *testing.T) {
	targetRelease := publishTestRelease(t)

	evidence, err := publishEvidence(
		targetRelease,
		"lvinst_prod",
		"prod",
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.ArtifactDigest != targetRelease.Provenance.ArtifactDigest ||
		evidence.PlanDigest != targetRelease.Provenance.PlanDigest ||
		evidence.ReleaseDigest != targetRelease.Provenance.Digest ||
		evidence.CandidateID != "candidate_1" ||
		evidence.CandidateRevision != 4 ||
		evidence.TargetID != "lvinst_prod" {
		t.Fatalf("publish evidence = %#v", evidence)
	}
	response := publishEvidenceResponse(targetRelease)
	if len(response.Workspaces) != 1 ||
		len(response.Workspaces[0].ManagedDataPins) != 1 ||
		len(response.Workspaces[0].Bindings) != 1 ||
		response.Workspaces[0].Bindings[0].ValidatedVersion != "version_7" {
		t.Fatalf("redacted publish evidence response = %#v", response)
	}
}

func TestPublishEvidenceRejectsCrossTargetAndIncompleteRelease(t *testing.T) {
	tests := map[string]func(*release.Release){
		"missing provenance": func(value *release.Release) {
			value.Provenance = nil
		},
		"cross target": func(value *release.Release) {
			value.Provenance.Plan.TargetID = "lvinst_other"
		},
		"environment drift": func(value *release.Release) {
			value.Provenance.Plan.Environment = "staging"
		},
		"serving state drift": func(value *release.Release) {
			value.Artifacts[0].ServingStateID = "state_other"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			targetRelease := publishTestRelease(t)
			mutate(&targetRelease)
			_, err := publishEvidence(targetRelease, "lvinst_prod", "prod")
			if !errors.Is(err, deployment.ErrConflict) {
				t.Fatalf("error = %v, want deployment conflict", err)
			}
		})
	}
}

func TestRetryCreatesOneNewRequestForTheSameImmutableRelease(t *testing.T) {
	targetRelease := publishTestRelease(t)
	coordinator := &publishCoordinatorStub{
		rows: map[string]apiadapter.Deployment{
			"deployment_failed": {
				ID: "deployment_failed", Project: "project", Environment: "prod",
				Status: apiadapter.StatusFailed,
			},
		},
	}
	releases := &publishReleaseStub{
		targetRelease: targetRelease,
		deployments:   map[string]string{"deployment_failed": targetRelease.ID},
	}
	module := &Module{
		handler: deploymenthttp.NewHandler(deploymenthttp.Options{
			Coordinator: coordinator, InstanceEnvironment: "prod",
			CurrentPrincipal: func(*http.Request) (deploymenthttp.Principal, bool) {
				return deploymenthttp.Principal{ID: "publisher"}, true
			},
		}),
		instanceID: "lvinst_prod",
		jobs:       JobConfig{Coordinator: coordinator},
		api:        APIConfig{Releases: releases},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/project/deployments/deployment_failed/retry",
		nil,
	)

	module.RetryDeployment(
		recorder,
		request,
		"project",
		"deployment_failed",
		"retry-1",
	)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if coordinator.created.ReleaseID != targetRelease.ID ||
		coordinator.created.IdempotencyKey != "retry-1" ||
		coordinator.created.Evidence.PlanDigest != targetRelease.Provenance.PlanDigest ||
		coordinator.created.Evidence.ReleaseDigest != targetRelease.Provenance.Digest {
		t.Fatalf("retry request = %#v", coordinator.created)
	}
}

func publishTestRelease(t *testing.T) release.Release {
	t.Helper()
	artifactDigest := "sha256:" + strings.Repeat("a", 64)
	projectDigest := "sha256:" + strings.Repeat("b", 64)
	policyDigest := "sha256:" + strings.Repeat("c", 64)
	provenance, err := release.NewProvenance(release.ProvenanceInput{
		Artifact: release.ProjectArtifactProvenance{
			SourceDigest:  "sha256:" + strings.Repeat("d", 64),
			ProjectDigest: projectDigest, CompilerVersion: "test", SchemaVersion: 1,
			Workspaces: []release.WorkspaceArtifactProvenance{{
				WorkspaceID: "sales", ArtifactDigest: artifactDigest,
			}},
		},
		Candidate: release.CandidateProvenance{
			ID: "candidate_1", Revision: 4, OwnerID: "author_1",
		},
		Plan: release.TargetPlanProvenance{
			TargetID: "lvinst_prod", Environment: "prod",
			BaseGeneration: "deployment_3", RuntimeVersion: "test",
			PolicyDigest: policyDigest,
			Workspaces: []release.TargetWorkspacePlan{{
				WorkspaceID: "sales", ServingStateID: "state_4",
				ArtifactDigest: artifactDigest, DataRevision: "snapshot_4",
				DataMode: release.TargetDataRefreshSources,
				ManagedDataPins: []release.ManagedDataPin{{
					ConnectionID: "orders",
					RevisionID:   "sha256:" + strings.Repeat("e", 64),
				}},
				Bindings: []release.BindingEvidence{{
					BindingID: "warehouse", Revision: 7,
					ValidatedVersion: "version_7",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return release.Release{
		ID: "release_1", ProjectID: "project", ProjectDigest: projectDigest,
		Status: release.StatusReady, Provenance: &provenance,
		Artifacts: []release.Artifact{{
			ReleaseID: "release_1", WorkspaceID: "sales",
			ExpectedDigest: artifactDigest, ActualDigest: artifactDigest,
			ServingStateID: "state_4",
		}},
	}
}

type publishCoordinatorStub struct {
	rows    map[string]apiadapter.Deployment
	created apiadapter.CreateRequest
}

func (stub *publishCoordinatorStub) Create(
	_ context.Context,
	request apiadapter.CreateRequest,
) (apiadapter.Deployment, error) {
	stub.created = request
	return apiadapter.Deployment{
		ID: "deployment_retry", Project: request.Project,
		Environment: request.Environment, RequestDigest: request.Evidence.PlanDigest,
		Status: apiadapter.StatusPending,
	}, nil
}

func (stub *publishCoordinatorStub) Get(
	_ context.Context,
	scope apiadapter.Scope,
) (apiadapter.Deployment, error) {
	row, ok := stub.rows[scope.DeploymentID]
	if !ok {
		return apiadapter.Deployment{}, deployment.ErrNotFound
	}
	return row, nil
}

func (*publishCoordinatorStub) Activate(
	context.Context,
	apiadapter.ActivateRequest,
) (apiadapter.Deployment, error) {
	return apiadapter.Deployment{}, nil
}

func (*publishCoordinatorStub) Cancel(
	context.Context,
	apiadapter.Scope,
) (apiadapter.Deployment, error) {
	return apiadapter.Deployment{}, nil
}

type publishReleaseStub struct {
	targetRelease release.Release
	deployments   map[string]string
}

func (stub *publishReleaseStub) Get(
	_ context.Context,
	projectID,
	releaseID string,
) (release.Release, error) {
	if projectID != stub.targetRelease.ProjectID ||
		releaseID != stub.targetRelease.ID {
		return release.Release{}, release.ErrNotFound
	}
	return stub.targetRelease, nil
}

func (*publishReleaseStub) LinkDeployment(
	context.Context,
	string,
	string,
	string,
	string,
) error {
	return nil
}

func (*publishReleaseStub) LinkDeploymentTx(
	context.Context,
	transaction.Transaction,
	string,
	string,
	string,
	string,
) error {
	return nil
}

func (stub *publishReleaseStub) DeploymentRelease(
	_ context.Context,
	projectID,
	deploymentID string,
) (string, string, error) {
	if projectID != stub.targetRelease.ProjectID {
		return "", "", release.ErrNotFound
	}
	releaseID, ok := stub.deployments[deploymentID]
	if !ok {
		return "", "", release.ErrNotFound
	}
	return releaseID, "", nil
}

func (*publishReleaseStub) ListDeploymentIDs(
	context.Context,
	string,
) ([]string, error) {
	return nil, nil
}

func (*publishReleaseStub) PriorDeploymentRelease(
	context.Context,
	string,
	string,
) (string, error) {
	return "", release.ErrNotFound
}
