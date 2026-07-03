package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/workspace"
)

type servingState struct {
	Deployment deployment.Deployment
	Artifact   deployment.Artifact
}

type servingStateService struct {
	repo       deploymentRepository
	drainGrace time.Duration
}

type servingRefreshCandidateInput struct {
	WorkspaceID   string
	Environment   deployment.Environment
	CreatedBy     string
	Active        servingState
	ArtifactGraph workspace.AssetGraph
}

func newServingStateService(repo deploymentRepository, drainGrace time.Duration) servingStateService {
	if drainGrace == 0 {
		drainGrace = deployment.DefaultRetentionPolicy().QueryDrainGrace
	}
	return servingStateService{repo: repo, drainGrace: drainGrace}
}

func (s servingStateService) Active(ctx context.Context, workspaceID string, environment deployment.Environment) (servingState, error) {
	active, artifact, err := s.repo.ActiveArtifact(ctx, deployment.WorkspaceID(workspaceID), environment)
	if err != nil {
		return servingState{}, err
	}
	return servingState{Deployment: active, Artifact: artifact}, nil
}

func (s servingStateService) CreateRefreshCandidate(ctx context.Context, input servingRefreshCandidateInput) (servingState, error) {
	active := input.Active
	workspaceID := deployment.WorkspaceID(input.WorkspaceID)
	environment := deployment.NormalizeEnvironment(input.Environment)
	created, err := s.repo.Create(ctx, deployment.CreateInput{
		WorkspaceID: workspaceID,
		Environment: environment,
		CreatedBy:   input.CreatedBy,
		Source:      deployment.SourceRefresh,
	})
	if err != nil {
		return servingState{}, err
	}
	validated, err := s.repo.SaveValidated(ctx, created.ID, deployment.Validation{
		Digest:       active.Deployment.Digest,
		ManifestJSON: active.Deployment.ManifestJSON,
		Graph:        retargetAssetGraph(input.ArtifactGraph, workspace.WorkspaceID(input.WorkspaceID), workspace.DeploymentID(created.ID)),
	}, deployment.Artifact{
		ID:           "artifact_" + string(created.ID),
		DeploymentID: created.ID,
		WorkspaceID:  workspaceID,
		Environment:  environment,
		Digest:       active.Artifact.Digest,
		Format:       active.Artifact.Format,
		Path:         active.Artifact.Path,
		ManifestJSON: active.Artifact.ManifestJSON,
		SizeBytes:    active.Artifact.SizeBytes,
		CreatedAt:    active.Artifact.CreatedAt,
	})
	if err != nil {
		_ = s.repo.MarkFailed(ctx, created.ID, err)
		return servingState{}, err
	}
	return servingState{Deployment: validated, Artifact: active.Artifact}, nil
}

func (s servingStateService) RecordSnapshot(ctx context.Context, candidate servingState, snapshotID int64) error {
	if snapshotID <= 0 {
		return fmt.Errorf("serving state snapshot id must be positive")
	}
	return s.repo.RecordDuckLakeSnapshot(ctx, candidate.Deployment.ID, snapshotID)
}

func (s servingStateService) Activate(ctx context.Context, candidate servingState, cleanupAfter time.Time) (deployment.Deployment, error) {
	return s.MarkSuperseded(ctx, candidate, cleanupAfter)
}

func (s servingStateService) MarkSuperseded(ctx context.Context, replacement servingState, cleanupAfter time.Time) (deployment.Deployment, error) {
	if cleanupAfter.IsZero() {
		cleanupAfter = time.Now().Add(s.drainGrace)
	}
	if activator, ok := s.repo.(servingStateActivator); ok {
		return activator.ActivateWithCleanupAfter(ctx, replacement.Deployment.WorkspaceID, replacement.Deployment.Environment, replacement.Deployment.ID, cleanupAfter)
	}
	return s.repo.Activate(ctx, replacement.Deployment.WorkspaceID, replacement.Deployment.Environment, replacement.Deployment.ID)
}

func (s servingStateService) MarkFailed(ctx context.Context, state servingState, cause error) error {
	if state.Deployment.ID == "" || cause == nil {
		return nil
	}
	return s.repo.MarkFailed(ctx, state.Deployment.ID, cause)
}

func (s servingStateService) ReconcileRetention(ctx context.Context, now time.Time) error {
	reconciler, ok := s.repo.(servingRetentionReconciler)
	if !ok {
		return nil
	}
	return reconciler.ReconcileRetention(ctx, now)
}
