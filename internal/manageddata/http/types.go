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
	ErrInvalid  = control.ErrInvalid
	ErrNotFound = control.ErrNotFound
	ErrConflict = control.ErrConflict
	ErrTooLarge = errors.New("managed-data request is too large")
	ErrBackend  = control.ErrBackend
)

type Principal struct {
	ID string
}

type RevisionMetadata = control.RevisionMetadata
type Repository = control.MetadataRepository

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

type Options struct {
	Repository       Repository
	Uploads          UploadCoordinator
	Multipart        MultipartCoordinator
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
