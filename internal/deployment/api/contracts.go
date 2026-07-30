package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type PageParams struct {
	Limit     *int32
	PageToken *string
}

type CandidateStartRequest struct {
	ArtifactDigest string `json:"artifactDigest"`
}

type CandidateArtifactRequest struct {
	ExpectedArtifactDigest string `json:"expectedArtifactDigest"`
	ArtifactDigest         string `json:"artifactDigest"`
}

type CandidateSourceArtifact struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type CandidateSynchronizationRequest struct {
	ProjectFile            string                    `json:"projectFile"`
	ArtifactDigest         string                    `json:"artifactDigest"`
	ExpectedCandidateID    *string                   `json:"expectedCandidateId,omitempty"`
	ExpectedArtifactDigest *string                   `json:"expectedArtifactDigest,omitempty"`
	Artifacts              []CandidateSourceArtifact `json:"artifacts"`
}

type CandidateSynchronizationPlanResponse struct {
	ArtifactDigest string   `json:"artifactDigest"`
	MissingDigests []string `json:"missingDigests"`
}

type CandidateSourceBlobResponse struct {
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes"`
}

type CandidateResponse struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"projectId"`
	TargetID         string  `json:"targetId"`
	Environment      string  `json:"environment"`
	OwnerID          string  `json:"ownerId"`
	BaseGeneration   string  `json:"baseGeneration"`
	ArtifactDigest   string  `json:"artifactDigest"`
	ProvenanceDigest *string `json:"provenanceDigest,omitempty"`
	Status           string  `json:"status"`
	FailureReason    *string `json:"failureReason,omitempty"`
	PreviewURL       string  `json:"previewUrl"`
	ExpiresAt        string  `json:"expiresAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	Revision         int64   `json:"revision"`
	Resumed          *bool   `json:"resumed,omitempty"`
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
