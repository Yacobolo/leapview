package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type PageParams struct {
	Limit     *int32
	PageToken *string
}

type CreateRequest struct {
	ReleaseID string `json:"releaseId"`
}

type Status string

const StatusQueued Status = "queued"

type TargetResponse struct {
	Error               *string `json:"error,omitempty"`
	PriorServingStateID *string `json:"priorServingStateId,omitempty"`
	ServingStateID      *string `json:"servingStateId,omitempty"`
	Status              string  `json:"status"`
	WorkspaceID         string  `json:"workspaceId"`
}

type ConnectionResponse struct {
	ConnectionID    string  `json:"connectionId"`
	PriorRevisionID *string `json:"priorRevisionId,omitempty"`
	RevisionID      string  `json:"revisionId"`
}

type Response struct {
	Connections []ConnectionResponse `json:"connections"`
	CreatedAt   string               `json:"createdAt"`
	CreatedBy   string               `json:"createdBy"`
	Environment string               `json:"environment"`
	Error       *string              `json:"error,omitempty"`
	FinishedAt  *string              `json:"finishedAt,omitempty"`
	ID          string               `json:"id"`
	ProjectID   string               `json:"projectId"`
	ReleaseID   string               `json:"releaseId"`
	StartedAt   *string              `json:"startedAt,omitempty"`
	Status      Status               `json:"status"`
	Targets     []TargetResponse     `json:"targets"`
}

type ListResponse struct {
	Items []Response `json:"items"`
	Page  PageInfo   `json:"page"`
}
