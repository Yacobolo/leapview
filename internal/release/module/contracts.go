package module

import "github.com/flidai/leapview/internal/release"

type Provenance = release.Provenance

var (
	ErrNotFound          = release.ErrNotFound
	ErrProvenanceInvalid = release.ErrProvenanceInvalid
)
