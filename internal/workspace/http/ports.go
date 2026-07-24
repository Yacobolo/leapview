package http

import (
	"context"
	nethttp "net/http"

	"github.com/Yacobolo/leapview/internal/workspace"
	"github.com/Yacobolo/leapview/internal/workspace/navigation"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
)

type Metrics interface {
	Catalog() navigation.Catalog
	DataExplorerModel(modelID string) (DataExplorerModel, bool)
	ExecuteDataPreview(ctx context.Context, request DataPreviewRequest) (DataPreviewResult, error)
}

type DataPreviewRequest struct {
	WorkspaceID  string
	ObjectKey    string
	Layer        string
	ModelID      string
	Table        string
	Columns      []string
	SortColumn   string
	Direction    string
	Offset       int
	Limit        int
	IncludeTotal bool
}

type DataPreviewResult struct {
	Rows           []map[string]any
	TotalRows      int
	TotalRowsKnown bool
	SQL            string
}

type DataExplorerModel struct {
	Sources map[string]DataExplorerSource
	Tables  map[string]DataExplorerTable
}

type DataExplorerSource struct {
	Fields  map[string]DataExplorerField
	Columns []DataExplorerColumn
}

type DataExplorerTable struct {
	Dimensions map[string]DataExplorerField
	Columns    map[string]DataExplorerField
	Schema     []DataExplorerColumn
}

type DataExplorerField struct {
	Name  string
	Label string
	Type  string
}

type DataExplorerColumn struct {
	Name         string
	PhysicalType string
	Ordinal      int
}

type AssetCatalogReader interface {
	ActiveAssetCatalog(ctx context.Context, workspaceID workspace.WorkspaceID, environment string) (workspace.AssetCatalog, bool, error)
}

type RefreshStateProvider interface {
	AssetRefreshState(ctx context.Context, workspaceID, environment string, asset workspace.AssetView) (ui.AssetRefreshState, error)
	AssetVersionsState(ctx context.Context, workspaceID, environment string, asset workspace.AssetView, section string) (ui.AssetVersionsState, error)
}

type AssetRefreshRunner interface {
	RefreshAsset(ctx context.Context, input AssetRefreshInput) error
}

type AssetRefreshInput struct {
	Request     *nethttp.Request
	WorkspaceID string
	Asset       workspace.AssetView
	Assets      []workspace.AssetView
	Edges       []workspace.AssetEdgeView
}
