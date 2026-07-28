package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type PageParams struct {
	Limit     *int32
	PageToken *string
}

type WorkspaceManifest struct {
	ArtifactDigest string  `json:"artifactDigest"`
	ServingStateID *string `json:"servingStateId,omitempty"`
	Workspace      string  `json:"workspace"`
}

type ConnectionPin struct {
	Connection string `json:"connection"`
	RevisionID string `json:"revisionId"`
}

type CreateRequest struct {
	Connections   []ConnectionPin     `json:"connections"`
	ProjectDigest string              `json:"projectDigest"`
	Workspaces    []WorkspaceManifest `json:"workspaces"`
}

type Status string

type Response struct {
	Connections   []ConnectionPin     `json:"connections"`
	CreatedAt     string              `json:"createdAt"`
	CreatedBy     string              `json:"createdBy"`
	Error         *string             `json:"error,omitempty"`
	FinalizedAt   *string             `json:"finalizedAt,omitempty"`
	ID            string              `json:"id"`
	ProjectDigest string              `json:"projectDigest"`
	ProjectID     string              `json:"projectId"`
	Status        Status              `json:"status"`
	Workspaces    []WorkspaceManifest `json:"workspaces"`
}

type ListResponse struct {
	Items []Response `json:"items"`
	Page  PageInfo   `json:"page"`
}

type ArtifactResponse struct {
	Digest      string `json:"digest"`
	ReleaseID   string `json:"releaseId"`
	SizeBytes   int64  `json:"sizeBytes"`
	WorkspaceID string `json:"workspaceId"`
}

type ManagedConnectionResponse struct {
	ActiveRevisionID *string `json:"activeRevisionId,omitempty"`
	Description      *string `json:"description,omitempty"`
	ID               string  `json:"id"`
	ProjectID        string  `json:"projectId"`
	Title            string  `json:"title"`
}

type ManagedConnectionListResponse struct {
	Items []ManagedConnectionResponse `json:"items"`
	Page  PageInfo                    `json:"page"`
}
