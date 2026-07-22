// Package publication owns anonymous dashboard publication lifecycle state.
package publication

import (
	"errors"

	"github.com/Yacobolo/leapview/internal/workspace"
)

var (
	ErrNotFound               = errors.New("dashboard publication not found")
	ErrConflict               = errors.New("dashboard publication conflict")
	ErrStreamStateUnavailable = errors.New("durable publication stream state is unavailable")
)

type Status string

const (
	StatusActive       Status = "active"
	StatusSuspended    Status = "suspended"
	StatusUnconfigured Status = "unconfigured"
)

type Publication struct {
	ID                  string
	ProjectID           string
	WorkspaceID         string
	Name                string
	PublicID            string
	Dashboard           string
	DefaultPage         string
	ConfigurationDigest string
	AllowedOrigins      []string
	DependencyAssetIDs  []string
	Configured          bool
	ServingStateID      string
	SuspendedAt         string
	SuspendedBy         string
	ConfiguredAt        string
	DisabledAt          string
	RotatedAt           string
	CreatedAt           string
	UpdatedAt           string
}

func (p Publication) Status() Status {
	if !p.Configured || p.ServingStateID == "" {
		return StatusUnconfigured
	}
	if p.SuspendedAt != "" {
		return StatusSuspended
	}
	return StatusActive
}

type ReconcileInput struct {
	ProjectID      string
	WorkspaceID    string
	ServingStateID string
	ActorID        string
	Publications   map[string]workspace.DashboardPublication
}

type Event struct {
	Type           string
	ActorID        string
	ServingStateID string
	CreatedAt      string
}
