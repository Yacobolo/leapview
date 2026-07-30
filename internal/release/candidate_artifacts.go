package release

import (
	"context"
	"errors"

	"github.com/flidai/leapview/internal/project"
)

var (
	ErrCandidateArtifactInvalid     = errors.New("candidate artifact invalid")
	ErrCandidateArtifactUnavailable = errors.New("candidate artifact preparation unavailable")
)

type CandidateConnectionRequirement struct {
	LogicalConnectionID string
	ConnectorKind       string
}

type CandidateRestriction struct {
	ID             string
	WorkspaceID    string
	ObjectID       string
	PolicyType     string
	ExpressionJSON string
}

type CandidateArtifactWorkspace struct {
	WorkspaceID     string
	ServingStateID  string
	ArtifactDigest  string
	DataRevision    string
	DataMode        string
	ManagedDataPins []ManagedDataPin
	Connections     []CandidateConnectionRequirement
	Restrictions    []CandidateRestriction
}

type CandidateArtifactRequest struct {
	CandidateID    string
	ProjectID      string
	OwnerID        string
	Environment    string
	ArtifactDigest string
	Source         project.CandidateSourceSnapshot
}

type CandidateArtifactSet struct {
	Artifact                 ProjectArtifactProvenance
	AuthorizationFingerprint string
	Workspaces               []CandidateArtifactWorkspace
}

type CandidateArtifactPreparer interface {
	PrepareCandidateArtifacts(context.Context, CandidateArtifactRequest) (CandidateArtifactSet, error)
	RetainCandidateProvenance(
		context.Context,
		string,
		Provenance,
	) (Provenance, error)
	CandidateProvenance(
		context.Context,
		string,
		string,
		int64,
	) (Provenance, error)
}
