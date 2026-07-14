// Package http exposes managed-data control operations over the generated API.
package http

import (
	"context"
	"errors"
	stdhttp "net/http"

	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/Yacobolo/libredash/internal/manageddata/control"
)

var (
	ErrInvalid  = errors.New("invalid managed-data request")
	ErrNotFound = errors.New("managed-data resource not found")
	ErrConflict = errors.New("managed-data conflict")
	ErrTooLarge = errors.New("managed-data request is too large")
	ErrBackend  = errors.New("managed-data backend is unavailable")
)

type Principal struct {
	ID string
}

// RevisionMetadata carries the upload provenance required by the public API.
// Persistence adapters should populate it with a scoped metadata query rather
// than exposing storage records to the transport layer.
type RevisionMetadata struct {
	Revision        manageddata.Revision
	UploadSessionID string
}

type Repository interface {
	CollectionByProjectConnection(context.Context, string, string) (manageddata.Collection, error)
	RevisionByID(context.Context, string) (RevisionMetadata, error)
	ListRevisions(context.Context, string) ([]RevisionMetadata, error)
	EnvironmentPointer(context.Context, string, manageddata.Environment) (manageddata.EnvironmentPointer, error)
}

type UploadCoordinator interface {
	BeginUpload(context.Context, control.BeginUploadRequest) (control.UploadResult, error)
	RecoverUpload(context.Context, control.UploadRequest) (control.UploadResult, error)
	FinalizeUpload(context.Context, control.UploadRequest) (control.FinalizeResult, error)
	AbortUpload(context.Context, control.UploadRequest) (control.UploadResult, error)
}

type MultipartStatus string

const (
	MultipartStatusOpen      MultipartStatus = "open"
	MultipartStatusCompleted MultipartStatus = "completed"
	MultipartStatusAborted   MultipartStatus = "aborted"
)

type MultipartUpload struct {
	ID              string
	UploadSessionID string
	File            manageddata.File
	Status          MultipartStatus
	Existing        bool
	CreatedAt       string
	ExpiresAt       string
}

type MultipartCreateRequest struct {
	Project         string
	Connection      string
	CollectionID    string
	UploadSessionID string
	File            manageddata.File
	Actor           string
	IdempotencyKey  string
}

type MultipartRequest struct {
	Project           string
	Connection        string
	CollectionID      string
	UploadSessionID   string
	MultipartUploadID string
	Actor             string
	IdempotencyKey    string
}

type MultipartSignPartRequest struct {
	MultipartRequest
	PartNumber int32
	Size       int64
	SHA256     string
}

type CompletedPart struct {
	PartNumber int32
	ETag       string
	SHA256     string
}

type MultipartCompleteRequest struct {
	MultipartRequest
	Parts []CompletedPart
}

type HTTPHeader struct {
	Name  string
	Value string
}

type MultipartSignedPart struct {
	UploadSessionID   string
	MultipartUploadID string
	PartNumber        int32
	URL               string
	Headers           []HTTPHeader
	ExpiresAt         string
}

type MultipartCoordinator interface {
	Create(context.Context, MultipartCreateRequest) (MultipartUpload, error)
	SignPart(context.Context, MultipartSignPartRequest) (MultipartSignedPart, error)
	Complete(context.Context, MultipartCompleteRequest) (MultipartUpload, error)
	Abort(context.Context, MultipartRequest) (MultipartUpload, error)
}

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

type RolloutCoordinator interface {
	List(context.Context, RolloutListRequest) ([]Rollout, error)
	Get(context.Context, RolloutRequest) (Rollout, error)
	Create(context.Context, RolloutCreateRequest) (Rollout, error)
	Activate(context.Context, RolloutRequest) (Rollout, error)
	Rollback(context.Context, RolloutRollbackRequest) (Rollout, error)
}

type Options struct {
	Repository       Repository
	Uploads          UploadCoordinator
	Multipart        MultipartCoordinator
	Rollouts         RolloutCoordinator
	CurrentPrincipal func(*stdhttp.Request) (Principal, bool)
	MaxJSONBodyBytes int64
}

type Handler struct {
	options Options
}

func NewHandler(options Options) *Handler {
	if options.MaxJSONBodyBytes <= 0 {
		options.MaxJSONBodyBytes = 16 << 20
	}
	return &Handler{options: options}
}
