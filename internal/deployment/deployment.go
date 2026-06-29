package deployment

import (
	"errors"

	"github.com/Yacobolo/libredash/internal/workspace"
)

var ErrNotFound = errors.New("deployment not found")

type ID string

type WorkspaceID string

type Status string

const (
	StatusPending   Status = "pending"
	StatusValidated Status = "validated"
	StatusActive    Status = "active"
	StatusInactive  Status = "inactive"
	StatusFailed    Status = "failed"
)

type Deployment struct {
	ID           ID
	WorkspaceID  WorkspaceID
	Status       Status
	Digest       string
	ManifestJSON string
	CreatedBy    string
	CreatedAt    string
	ActivatedAt  string
	Error        string
}

func (d Deployment) CanActivate() bool {
	return d.Status == StatusValidated || d.Status == StatusInactive || d.Status == StatusActive
}

type CreateInput struct {
	WorkspaceID WorkspaceID
	CreatedBy   string
}

type Artifact struct {
	ID           string
	DeploymentID ID
	WorkspaceID  WorkspaceID
	Digest       string
	Format       string
	Path         string
	ManifestJSON string
	SizeBytes    int64
	CreatedAt    string
}

type Validation struct {
	Digest       string
	ManifestJSON string
	RootDir      string
	Graph        workspace.AssetGraph
}

type PreparedRuntime interface {
	Close() error
}
