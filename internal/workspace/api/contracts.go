package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type WorkspaceResponse struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Description          string `json:"description"`
	ActiveServingStateID string `json:"activeServingStateId,omitempty"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type SearchParams struct {
	Query            *string
	Workspaces       *[]string
	Types            *[]string
	ContextWorkspace *string
	ContextDashboard *string
	ContextPage      *string
	Limit            *int32
	PageToken        *string
}

type SearchContextTag string

type SearchLocation struct {
	DashboardID   *string `json:"dashboardId,omitempty"`
	DashboardName *string `json:"dashboardName,omitempty"`
	Href          string  `json:"href"`
	PageID        *string `json:"pageId,omitempty"`
	PageName      *string `json:"pageName,omitempty"`
}

type SearchReference struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	WorkspaceID string `json:"workspaceId"`
}

type SearchWorkspaceSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SearchResult struct {
	Context     []SearchContextTag     `json:"context"`
	Description *string                `json:"description,omitempty"`
	Href        string                 `json:"href"`
	Locations   []SearchLocation       `json:"locations"`
	Name        string                 `json:"name"`
	Reference   SearchReference        `json:"reference"`
	VisualType  *string                `json:"visualType,omitempty"`
	Workspace   SearchWorkspaceSummary `json:"workspace"`
}

type SearchResponse struct {
	Items []SearchResult `json:"items"`
	Page  PageInfo       `json:"page"`
}

type AssetResponse struct {
	ID             string         `json:"id"`
	SnapshotID     string         `json:"snapshotId"`
	WorkspaceID    string         `json:"workspaceId"`
	ServingStateID string         `json:"servingStateId"`
	Type           string         `json:"type"`
	Key            string         `json:"key"`
	ParentID       string         `json:"parentId,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	SourceFile     string         `json:"sourceFile,omitempty"`
	PayloadSchema  string         `json:"payloadSchema"`
	Payload        map[string]any `json:"payload"`
	Href           string         `json:"href,omitempty"`
}

type AssetSummaryResponse struct {
	ID             string `json:"id"`
	SnapshotID     string `json:"snapshotId"`
	WorkspaceID    string `json:"workspaceId"`
	ServingStateID string `json:"servingStateId"`
	Type           string `json:"type"`
	Key            string `json:"key"`
	ParentID       string `json:"parentId,omitempty"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	SourceFile     string `json:"sourceFile,omitempty"`
	PayloadSchema  string `json:"payloadSchema"`
	ContentHash    string `json:"contentHash"`
	Href           string `json:"href,omitempty"`
}

type AssetGraphAssetResponse struct {
	ID             string         `json:"id"`
	SnapshotID     string         `json:"snapshotId"`
	WorkspaceID    string         `json:"workspaceId"`
	ServingStateID string         `json:"servingStateId"`
	Type           string         `json:"type"`
	Key            string         `json:"key"`
	ParentID       string         `json:"parentId,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	SourceFile     string         `json:"sourceFile,omitempty"`
	PayloadSchema  string         `json:"payloadSchema"`
	Payload        map[string]any `json:"payload"`
	ContentHash    string         `json:"contentHash"`
}

type WorkspaceAssetGraphResponse struct {
	Assets []AssetGraphAssetResponse `json:"assets"`
	Edges  []AssetEdgeResponse       `json:"edges"`
}

type AssetEdgeResponse struct {
	ID             string `json:"id"`
	WorkspaceID    string `json:"workspaceId"`
	ServingStateID string `json:"servingStateId"`
	FromAssetID    string `json:"fromAssetId"`
	ToAssetID      string `json:"toAssetId"`
	Type           string `json:"type"`
}

type AssetLineageResponse struct {
	AssetID    string   `json:"assetId"`
	Upstream   []string `json:"upstream"`
	Downstream []string `json:"downstream"`
}
