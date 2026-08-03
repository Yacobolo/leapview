package runtimefactory

import (
	"context"

	dashboardruntime "github.com/flidai/leapview/internal/dashboard/runtime"
	"github.com/flidai/leapview/internal/workspace"
)

func (r dashboardRuntimeWithGraph) Verify(ctx context.Context) error {
	return r.Service.Verify(ctx)
}

type dashboardRuntimeWithGraph struct {
	*dashboardruntime.Service
	workspaceID    string
	servingStateID string
	graph          workspace.AssetGraph
}

func (r dashboardRuntimeWithGraph) WorkspaceAssets(workspaceID, servingStateID string) ([]workspace.Asset, []workspace.AssetEdge, bool) {
	if r.workspaceID != workspaceID || r.servingStateID != servingStateID {
		return nil, nil, false
	}
	return append([]workspace.Asset(nil), r.graph.Assets...), append([]workspace.AssetEdge(nil), r.graph.Edges...), true
}
