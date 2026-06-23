package workspace

type Workspace struct {
	ID          WorkspaceID
	Title       string
	Description string
	BaseDir     string
	Graph       AssetGraph
}
