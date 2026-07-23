package release

import "context"

type ProjectRecord struct {
	ID, CreatedAt, UpdatedAt, LatestReleaseID, ActiveDeploymentID string
}

type WorkspaceRecord struct {
	ID, Title, Description, ActiveServingStateID string
}

type ConnectionRecord struct {
	ID, Title, Description, ActiveRevisionID string
}

type CatalogRepository interface {
	ListProjects(context.Context) ([]ProjectRecord, error)
	GetProject(context.Context, string) (ProjectRecord, error)
	ListProjectWorkspaces(context.Context, string, string) ([]WorkspaceRecord, error)
	ListConnections(context.Context, string, string) ([]ConnectionRecord, error)
	GetConnection(context.Context, string, string, string) (ConnectionRecord, error)
}

type DeploymentLinkage interface {
	Get(context.Context, string, string) (Release, error)
	LinkDeployment(context.Context, string, string, string, string) error
	DeploymentRelease(context.Context, string, string) (string, string, error)
	ListDeploymentIDs(context.Context, string) ([]string, error)
	PriorDeploymentRelease(context.Context, string, string) (string, error)
}
