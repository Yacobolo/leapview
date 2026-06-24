package app

import (
	"context"

	"github.com/Yacobolo/libredash/internal/workspace"
)

func ReconcileActiveLineageGraph(ctx context.Context, repo workspace.Repository, provider runtimeProvider, workspaceID string) error {
	if repo == nil || provider == nil || workspaceID == "" {
		return nil
	}
	graph, ok, err := repo.ActiveDeploymentGraph(ctx, workspace.WorkspaceID(workspaceID))
	if err != nil || !ok || !staleActiveLineageGraph(graph) {
		return err
	}
	refreshed, ok := activeWorkspaceGraph(provider, workspaceID, string(activeGraphDeploymentID(graph)))
	if !ok || staleActiveLineageGraph(refreshed) {
		return nil
	}
	return repo.ReplaceActiveDeploymentGraph(ctx, workspace.WorkspaceID(workspaceID), refreshed)
}

func activeWorkspaceGraph(provider runtimeProvider, workspaceID, deploymentID string) (workspace.AssetGraph, bool) {
	if provider == nil || deploymentID == "" {
		return workspace.AssetGraph{}, false
	}
	runtime, err := provider.Active()
	if err != nil {
		return workspace.AssetGraph{}, false
	}
	port, ok := runtime.(workspaceAssetRuntime)
	if !ok {
		return workspace.AssetGraph{}, false
	}
	assets, edges, ok := port.WorkspaceAssets(workspaceID, deploymentID)
	if !ok {
		return workspace.AssetGraph{}, false
	}
	return workspace.AssetGraph{Assets: assets, Edges: edges}, true
}
