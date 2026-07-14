package control

import "context"

type RolloutStatus string

const (
	RolloutStatusDraft       RolloutStatus = "draft"
	RolloutStatusActivating  RolloutStatus = "activating"
	RolloutStatusActive      RolloutStatus = "active"
	RolloutStatusFailed      RolloutStatus = "failed"
	RolloutStatusRollingBack RolloutStatus = "rolling_back"
	RolloutStatusRolledBack  RolloutStatus = "rolled_back"
)

type RolloutTargetStatus string

const (
	RolloutTargetStatusPending     RolloutTargetStatus = "pending"
	RolloutTargetStatusActivating  RolloutTargetStatus = "activating"
	RolloutTargetStatusActive      RolloutTargetStatus = "active"
	RolloutTargetStatusFailed      RolloutTargetStatus = "failed"
	RolloutTargetStatusRollingBack RolloutTargetStatus = "rolling_back"
	RolloutTargetStatusRolledBack  RolloutTargetStatus = "rolled_back"
)

type RolloutTarget struct {
	Workspace          string
	ServingStateID     string
	Status             RolloutTargetStatus
	PreviousRevisionID string
	ActivatedAt        string
	RolledBackAt       string
	Error              string
}

type Rollout struct {
	ID           string
	CollectionID string
	RevisionID   string
	Environment  string
	Status       RolloutStatus
	Targets      []RolloutTarget
	CreatedAt    string
	ActivatedAt  string
	RolledBackAt string
	Error        string
}

type RolloutListRequest struct {
	Project      string
	Connection   string
	CollectionID string
	Environment  string
	Status       RolloutStatus
}

type RolloutRequest struct {
	Project        string
	Connection     string
	CollectionID   string
	RolloutID      string
	Actor          string
	IdempotencyKey string
}

type RolloutTargetRequest struct {
	Workspace      string
	ServingStateID string
}

type RolloutCreateRequest struct {
	Project        string
	Connection     string
	CollectionID   string
	RevisionID     string
	Environment    string
	Targets        []RolloutTargetRequest
	Actor          string
	IdempotencyKey string
}

type RolloutRollbackRequest struct {
	RolloutRequest
	Reason string
}

// RolloutCoordinator owns the transport-neutral rollout control contract.
type RolloutCoordinator interface {
	List(context.Context, RolloutListRequest) ([]Rollout, error)
	Get(context.Context, RolloutRequest) (Rollout, error)
	Create(context.Context, RolloutCreateRequest) (Rollout, error)
	Activate(context.Context, RolloutRequest) (Rollout, error)
	Rollback(context.Context, RolloutRollbackRequest) (Rollout, error)
}
