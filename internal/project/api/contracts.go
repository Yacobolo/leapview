package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type ProjectResponse struct {
	ActiveDeploymentID *string `json:"activeDeploymentId,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	ID                 string  `json:"id"`
	LatestReleaseID    *string `json:"latestReleaseId,omitempty"`
	Title              string  `json:"title"`
	UpdatedAt          string  `json:"updatedAt"`
}

type ProjectListResponse struct {
	Items []ProjectResponse `json:"items"`
	Page  PageInfo          `json:"page"`
}

type ProjectWorkspaceResponse struct {
	ActiveServingStateID *string `json:"activeServingStateId,omitempty"`
	Description          *string `json:"description,omitempty"`
	ID                   string  `json:"id"`
	Title                string  `json:"title"`
}

type ProjectWorkspaceListResponse struct {
	Items []ProjectWorkspaceResponse `json:"items"`
	Page  PageInfo                   `json:"page"`
}
