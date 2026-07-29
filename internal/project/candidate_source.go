package project

import (
	"context"
	"errors"
	"io"
)

var (
	ErrCandidateSourceUnavailable = errors.New("candidate source synchronization unavailable")
	ErrCandidateSourceInvalid     = errors.New("candidate source synchronization invalid")
	ErrCandidateSourceConflict    = errors.New("candidate source synchronization conflict")
)

type CandidateSourceArtifact struct {
	Path   string
	Digest string
}

type CandidateSynchronizationRequest struct {
	ProjectFile            string
	ArtifactDigest         string
	ExpectedCandidateID    string
	ExpectedArtifactDigest string
	Artifacts              []CandidateSourceArtifact
}

type CandidateSourceScope struct {
	ProjectID string
	OwnerID   string
}

type CandidateSourceSnapshot struct {
	ProjectID           string
	ArtifactDigest      string
	ProjectPath         string
	ProjectDigest       string
	ProjectArtifactPath string
}

// CandidateSourceSynchronizer owns target-side retention and compiler
// validation for environment-neutral project sources.
type CandidateSourceSynchronizer interface {
	Plan(context.Context, CandidateSourceScope, CandidateSynchronizationRequest) ([]string, error)
	Upload(context.Context, CandidateSourceScope, string, io.Reader) error
	Commit(context.Context, CandidateSourceScope, CandidateSynchronizationRequest) (CandidateSourceSnapshot, error)
}
