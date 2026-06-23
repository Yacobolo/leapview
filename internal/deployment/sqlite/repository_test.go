package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/platform"
)

func TestRepositorySaveValidatedCommitsDeploymentGraph(t *testing.T) {
	ctx := context.Background()
	store, repo := openRepo(t, ctx)
	if err := store.EnsureWorkspace(ctx, platform.WorkspaceInput{ID: "test", Title: "Test"}); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	created, err := repo.Create(ctx, deployment.CreateInput{WorkspaceID: "test", CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	validation := validationGraph(created.ID, "edge_1", "edge_2")
	artifact := artifact(created.ID, "test")
	saved, err := repo.SaveValidated(ctx, created.ID, validation, artifact)
	if err != nil {
		t.Fatalf("save validated: %v", err)
	}
	if saved.Status != deployment.StatusValidated || saved.Digest != "digest" {
		t.Fatalf("saved = %#v, want validated digest", saved)
	}
	gotArtifact, err := repo.ArtifactByDeployment(ctx, created.ID)
	if err != nil {
		t.Fatalf("artifact: %v", err)
	}
	if gotArtifact.Path != "artifact.tar.gz" {
		t.Fatalf("artifact path = %q, want artifact.tar.gz", gotArtifact.Path)
	}
}

func TestRepositorySaveValidatedRollsBackOnDuplicateEdge(t *testing.T) {
	ctx := context.Background()
	store, repo := openRepo(t, ctx)
	if err := store.EnsureWorkspace(ctx, platform.WorkspaceInput{ID: "test", Title: "Test"}); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	created, err := repo.Create(ctx, deployment.CreateInput{WorkspaceID: "test", CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	validation := validationGraph(created.ID, "edge_1", "edge_2")
	validation.Edges[1].FromAssetID = validation.Edges[0].FromAssetID
	validation.Edges[1].ToAssetID = validation.Edges[0].ToAssetID
	validation.Edges[1].Type = validation.Edges[0].Type
	if _, err := repo.SaveValidated(ctx, created.ID, validation, artifact(created.ID, "test")); err == nil {
		t.Fatal("expected duplicate edge error")
	}

	after, err := repo.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after rollback: %v", err)
	}
	if after.Status != deployment.StatusPending {
		t.Fatalf("status = %q, want pending rollback", after.Status)
	}
	if _, err := repo.ArtifactByDeployment(ctx, created.ID); !errors.Is(err, deployment.ErrNotFound) {
		t.Fatalf("artifact error = %v, want ErrNotFound", err)
	}
}

func openRepo(t *testing.T, ctx context.Context) (*platform.Store, *Repository) {
	t.Helper()
	store, err := platform.Open(ctx, filepath.Join(t.TempDir(), "libredash.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, NewRepository(store.SQLDB())
}

func validationGraph(deploymentID deployment.ID, edgeID1, edgeID2 string) deployment.Validation {
	return deployment.Validation{
		Digest:       "digest",
		ManifestJSON: "{}",
		Assets: []deployment.Asset{
			{ID: "asset_a", WorkspaceID: "test", DeploymentID: deploymentID, Type: "dashboard", Key: "a", ContentJSON: "{}", ContentHash: "a"},
			{ID: "asset_b", WorkspaceID: "test", DeploymentID: deploymentID, Type: "semantic_model", Key: "b", ContentJSON: "{}", ContentHash: "b"},
		},
		Edges: []deployment.AssetEdge{
			{ID: edgeID1, WorkspaceID: "test", DeploymentID: deploymentID, FromAssetID: "asset_a", ToAssetID: "asset_b", Type: "uses_semantic_model"},
			{ID: edgeID2, WorkspaceID: "test", DeploymentID: deploymentID, FromAssetID: "asset_b", ToAssetID: "asset_a", Type: "contains"},
		},
	}
}

func artifact(deploymentID deployment.ID, workspaceID deployment.WorkspaceID) deployment.Artifact {
	return deployment.Artifact{
		ID:           "artifact_" + string(deploymentID),
		DeploymentID: deploymentID,
		WorkspaceID:  workspaceID,
		Digest:       "digest",
		Format:       "tar.gz",
		Path:         "artifact.tar.gz",
		ManifestJSON: "{}",
	}
}
