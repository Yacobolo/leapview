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

type ProjectArtifactWorkspace struct {
	WorkspaceID    string `json:"workspaceId"`
	ArtifactDigest string `json:"artifactDigest"`
}

type ProjectArtifactProvenance struct {
	SourceDigest    string                     `json:"sourceDigest"`
	ProjectDigest   string                     `json:"projectDigest"`
	CompilerVersion string                     `json:"compilerVersion"`
	SchemaVersion   int32                      `json:"schemaVersion"`
	Workspaces      []ProjectArtifactWorkspace `json:"workspaces"`
}

type CandidateProvenance struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	OwnerID  string `json:"ownerId"`
}

type ManagedDataPin struct {
	ConnectionID string `json:"connectionId"`
	RevisionID   string `json:"revisionId"`
}

type BindingEvidence struct {
	BindingID        string `json:"bindingId"`
	Revision         int64  `json:"revision"`
	ValidatedVersion string `json:"validatedVersion"`
}

type TargetWorkspacePlan struct {
	WorkspaceID     string            `json:"workspaceId"`
	ServingStateID  string            `json:"servingStateId"`
	ArtifactDigest  string            `json:"artifactDigest"`
	DataRevision    string            `json:"dataRevision"`
	DataMode        string            `json:"dataMode"`
	ManagedDataPins []ManagedDataPin  `json:"managedDataPins"`
	Bindings        []BindingEvidence `json:"bindings"`
}

type TargetPlanProvenance struct {
	TargetID       string                `json:"targetId"`
	Environment    string                `json:"environment"`
	BaseGeneration string                `json:"baseGeneration"`
	RuntimeVersion string                `json:"runtimeVersion"`
	PolicyDigest   string                `json:"policyDigest"`
	Workspaces     []TargetWorkspacePlan `json:"workspaces"`
}

type Provenance struct {
	Version        int32                     `json:"version"`
	Artifact       ProjectArtifactProvenance `json:"artifact"`
	Candidate      CandidateProvenance       `json:"candidate"`
	Plan           TargetPlanProvenance      `json:"plan"`
	ArtifactDigest string                    `json:"artifactDigest"`
	PlanDigest     string                    `json:"planDigest"`
	Digest         string                    `json:"digest"`
}

type CreateRequest struct {
	Connections   []ConnectionPin     `json:"connections"`
	ProjectDigest string              `json:"projectDigest"`
	Provenance    *Provenance         `json:"provenance,omitempty"`
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
	Provenance    *Provenance         `json:"provenance,omitempty"`
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
