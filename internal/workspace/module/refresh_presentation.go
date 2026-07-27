package module

import (
	"github.com/Yacobolo/leapview/internal/workspace"
	workspacedatastar "github.com/Yacobolo/leapview/internal/workspace/datastar"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type RefreshPresentation struct{}

func (RefreshPresentation) Sections() []string {
	return workspacedatastar.WorkspaceAssetRefreshSections()
}

func (RefreshPresentation) StreamID(workspaceID, assetID, section string) string {
	return workspacedatastar.WorkspaceAssetStreamID(workspaceID, assetID, section)
}

func (RefreshPresentation) Signals(view workspace.WorkspaceView, asset workspace.AssetView, assets []workspace.AssetView, edges []workspace.AssetEdgeView, refresh ui.AssetRefreshState, section string) pagestream.SignalPatch {
	return pagestream.SignalPatch(workspacedatastar.WorkspaceAssetRefreshSignals(view, asset, assets, edges, refresh, section))
}
