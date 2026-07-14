// Package apiadapter maps managed-data domain services to transport-neutral
// control contracts without exposing persistence identifiers or backend failures.
package apiadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/Yacobolo/libredash/internal/manageddata/control"
	"github.com/Yacobolo/libredash/internal/manageddata/rollout"
)

var canonicalRevisionID = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Repository is the persistence surface needed to present managed data through
// the control API. Revision lookup must reject ambiguous digests across collections.
type Repository interface {
	CollectionByProjectConnection(context.Context, string, string) (manageddata.Collection, error)
	RevisionByID(context.Context, string) (manageddata.Revision, error)
	ListRevisions(context.Context, string) ([]manageddata.Revision, error)
	UploadSessionIDByRevisionID(context.Context, string) (string, error)
	EnvironmentPointer(context.Context, string, manageddata.Environment) (manageddata.EnvironmentPointer, error)
	ListRollouts(context.Context, string) ([]manageddata.Rollout, error)
	ListServingStateBindings(context.Context, string) ([]manageddata.ServingStateBinding, error)
}

// RolloutService is the domain orchestration surface used by the adapter.
type RolloutService interface {
	Create(context.Context, rollout.CreateRequest) (manageddata.Rollout, error)
	Get(context.Context, rollout.Scope) (manageddata.Rollout, error)
	Activate(context.Context, rollout.Scope) (manageddata.Rollout, error)
	Rollback(context.Context, rollout.Scope, string) (manageddata.Rollout, error)
}

// Adapter implements the managed-data metadata and rollout control contracts.
type Adapter struct {
	repository Repository
	rollouts   RolloutService
}

func New(repository Repository, rollouts RolloutService) (*Adapter, error) {
	if repository == nil || rollouts == nil {
		return nil, fmt.Errorf("managed-data repository and rollout service are required")
	}
	return &Adapter{repository: repository, rollouts: rollouts}, nil
}

func (a *Adapter) CollectionByProjectConnection(ctx context.Context, project, connection string) (manageddata.Collection, error) {
	collection, err := a.repository.CollectionByProjectConnection(ctx, strings.TrimSpace(project), strings.TrimSpace(connection))
	if err != nil {
		return manageddata.Collection{}, publicError(err)
	}
	if collection.ProjectID != strings.TrimSpace(project) || collection.ConnectionName != strings.TrimSpace(connection) || collection.Status != manageddata.CollectionStatusActive {
		return manageddata.Collection{}, control.ErrNotFound
	}
	return collection, nil
}

// RevisionByID accepts only the public content-addressed revision identity.
func (a *Adapter) RevisionByID(ctx context.Context, collectionID, publicID string) (control.RevisionMetadata, error) {
	collectionID = strings.TrimSpace(collectionID)
	publicID = strings.TrimSpace(publicID)
	if collectionID == "" || !canonicalRevisionID.MatchString(publicID) {
		return control.RevisionMetadata{}, control.ErrInvalid
	}
	revision, err := a.scopedRevisionByDigest(ctx, collectionID, publicID)
	if err != nil {
		return control.RevisionMetadata{}, err
	}
	return a.revisionMetadata(ctx, revision)
}

func (a *Adapter) ListRevisions(ctx context.Context, collectionID string) ([]control.RevisionMetadata, error) {
	collectionID = strings.TrimSpace(collectionID)
	rows, err := a.repository.ListRevisions(ctx, collectionID)
	if err != nil {
		return nil, publicError(err)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Sequence == rows[j].Sequence {
			return rows[i].Digest > rows[j].Digest
		}
		return rows[i].Sequence > rows[j].Sequence
	})
	out := make([]control.RevisionMetadata, 0, len(rows))
	for _, revision := range rows {
		if revision.CollectionID != collectionID {
			return nil, control.ErrBackend
		}
		if revision.Status != manageddata.RevisionStatusReady {
			continue
		}
		metadata, metadataErr := a.revisionMetadata(ctx, revision)
		if metadataErr != nil {
			return nil, metadataErr
		}
		out = append(out, metadata)
	}
	return out, nil
}

func (a *Adapter) EnvironmentPointer(ctx context.Context, collectionID string, environment manageddata.Environment) (manageddata.EnvironmentPointer, error) {
	pointer, err := a.repository.EnvironmentPointer(ctx, strings.TrimSpace(collectionID), environment)
	if err != nil {
		return manageddata.EnvironmentPointer{}, publicError(err)
	}
	if pointer.CollectionID != collectionID || pointer.Environment != environment {
		return manageddata.EnvironmentPointer{}, control.ErrNotFound
	}
	revision, err := a.repository.RevisionByID(ctx, pointer.RevisionID)
	if err != nil {
		return manageddata.EnvironmentPointer{}, publicError(err)
	}
	if revision.CollectionID != collectionID || revision.Status != manageddata.RevisionStatusReady || !canonicalRevisionID.MatchString(revision.Digest) {
		return manageddata.EnvironmentPointer{}, control.ErrBackend
	}
	pointer.RevisionID = revision.Digest
	return pointer, nil
}

func (a *Adapter) List(ctx context.Context, request control.RolloutListRequest) ([]control.Rollout, error) {
	collection, err := a.requestCollection(ctx, request.Project, request.Connection, request.CollectionID)
	if err != nil {
		return nil, err
	}
	status, statusFilter, err := domainStatusFilter(request.Status)
	if err != nil {
		return nil, err
	}
	if statusFilter && status == "" {
		return []control.Rollout{}, nil
	}
	rows, err := a.repository.ListRollouts(ctx, collection.ID)
	if err != nil {
		return nil, publicError(err)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt == rows[j].CreatedAt {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].CreatedAt > rows[j].CreatedAt
	})
	out := make([]control.Rollout, 0, len(rows))
	for _, row := range rows {
		if row.CollectionID != collection.ID {
			return nil, control.ErrBackend
		}
		if request.Environment != "" && string(row.Environment) != request.Environment || statusFilter && row.Status != status {
			continue
		}
		mapped, mapErr := a.mapRollout(ctx, row)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (a *Adapter) Get(ctx context.Context, request control.RolloutRequest) (control.Rollout, error) {
	if _, err := a.requestCollection(ctx, request.Project, request.Connection, request.CollectionID); err != nil {
		return control.Rollout{}, err
	}
	row, err := a.rollouts.Get(ctx, rollout.Scope{Project: request.Project, Connection: request.Connection, RolloutID: request.RolloutID})
	if err != nil {
		return control.Rollout{}, publicError(err)
	}
	if row.ID != request.RolloutID || row.CollectionID != request.CollectionID {
		return control.Rollout{}, control.ErrNotFound
	}
	return a.mapRollout(ctx, row)
}

func (a *Adapter) Create(ctx context.Context, request control.RolloutCreateRequest) (control.Rollout, error) {
	collection, err := a.requestCollection(ctx, request.Project, request.Connection, request.CollectionID)
	if err != nil {
		return control.Rollout{}, err
	}
	if !canonicalRevisionID.MatchString(strings.TrimSpace(request.RevisionID)) || strings.TrimSpace(request.IdempotencyKey) == "" || strings.TrimSpace(request.Actor) == "" {
		return control.Rollout{}, control.ErrInvalid
	}
	revision, err := a.scopedRevisionByDigest(ctx, collection.ID, request.RevisionID)
	if err != nil {
		return control.Rollout{}, err
	}
	environment, err := manageddata.NormalizeEnvironment(request.Environment)
	if err != nil || len(request.Targets) == 0 {
		return control.Rollout{}, control.ErrInvalid
	}
	targets := make([]manageddata.RolloutTargetInput, len(request.Targets))
	seenWorkspaces := make(map[string]struct{}, len(request.Targets))
	for i, target := range request.Targets {
		workspace := strings.TrimSpace(target.Workspace)
		servingStateID := strings.TrimSpace(target.ServingStateID)
		if workspace == "" || servingStateID == "" {
			return control.Rollout{}, control.ErrInvalid
		}
		if _, exists := seenWorkspaces[workspace]; exists {
			return control.Rollout{}, control.ErrInvalid
		}
		seenWorkspaces[workspace] = struct{}{}
		targets[i] = manageddata.RolloutTargetInput{WorkspaceID: workspace, ServingStateID: servingStateID}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].WorkspaceID < targets[j].WorkspaceID })
	id := deterministicRolloutID(request.Project, request.Connection, request.Actor, request.IdempotencyKey)
	create := rollout.CreateRequest{
		ID: id, Project: request.Project, Connection: request.Connection, Environment: environment,
		RevisionID: revision.ID, Targets: targets, Actor: request.Actor,
	}
	row, err := a.rollouts.Create(ctx, create)
	if errors.Is(err, manageddata.ErrConflict) {
		row, err = a.rollouts.Get(ctx, rollout.Scope{Project: request.Project, Connection: request.Connection, RolloutID: id})
		if err == nil && !sameRolloutRequest(row, collection.ID, revision.ID, environment, targets, request.Actor) {
			return control.Rollout{}, control.ErrConflict
		}
	}
	if err != nil {
		return control.Rollout{}, publicError(err)
	}
	if row.ID != id || !sameRolloutRequest(row, collection.ID, revision.ID, environment, targets, request.Actor) {
		return control.Rollout{}, control.ErrBackend
	}
	return a.mapRollout(ctx, row)
}

func (a *Adapter) Activate(ctx context.Context, request control.RolloutRequest) (control.Rollout, error) {
	if _, err := a.requestCollection(ctx, request.Project, request.Connection, request.CollectionID); err != nil {
		return control.Rollout{}, err
	}
	row, err := a.rollouts.Activate(ctx, rollout.Scope{Project: request.Project, Connection: request.Connection, RolloutID: request.RolloutID})
	if err != nil {
		return control.Rollout{}, publicError(err)
	}
	if row.ID != request.RolloutID || row.CollectionID != request.CollectionID {
		return control.Rollout{}, control.ErrNotFound
	}
	return a.mapRollout(ctx, row)
}

func (a *Adapter) Rollback(ctx context.Context, request control.RolloutRollbackRequest) (control.Rollout, error) {
	if _, err := a.requestCollection(ctx, request.Project, request.Connection, request.CollectionID); err != nil {
		return control.Rollout{}, err
	}
	row, err := a.rollouts.Rollback(ctx, rollout.Scope{Project: request.Project, Connection: request.Connection, RolloutID: request.RolloutID}, request.Reason)
	if err != nil {
		return control.Rollout{}, publicError(err)
	}
	if row.CollectionID != request.CollectionID {
		return control.Rollout{}, control.ErrNotFound
	}
	mapped, err := a.mapRollout(ctx, row)
	if err != nil {
		return control.Rollout{}, err
	}
	// Rollback is implemented as a compensating rollout internally. The public
	// resource remains the rollout the operator requested to roll back.
	mapped.ID = request.RolloutID
	mapped.Status = control.RolloutStatusRolledBack
	mapped.RolledBackAt = row.CompletedAt
	for i := range mapped.Targets {
		mapped.Targets[i].Status = control.RolloutTargetStatusRolledBack
		mapped.Targets[i].RolledBackAt = row.CompletedAt
	}
	return mapped, nil
}

func (a *Adapter) requestCollection(ctx context.Context, project, connection, expectedID string) (manageddata.Collection, error) {
	collection, err := a.CollectionByProjectConnection(ctx, project, connection)
	if err != nil {
		return manageddata.Collection{}, err
	}
	if collection.ID != strings.TrimSpace(expectedID) {
		return manageddata.Collection{}, control.ErrNotFound
	}
	return collection, nil
}

func (a *Adapter) revisionMetadata(ctx context.Context, revision manageddata.Revision) (control.RevisionMetadata, error) {
	if revision.Status != manageddata.RevisionStatusReady || !canonicalRevisionID.MatchString(revision.Digest) {
		return control.RevisionMetadata{}, control.ErrNotFound
	}
	uploadID, err := a.repository.UploadSessionIDByRevisionID(ctx, revision.ID)
	if err != nil {
		return control.RevisionMetadata{}, publicError(err)
	}
	if strings.TrimSpace(uploadID) == "" {
		return control.RevisionMetadata{}, control.ErrBackend
	}
	revision.ID = revision.Digest
	return control.RevisionMetadata{Revision: revision, UploadSessionID: uploadID}, nil
}

func (a *Adapter) scopedRevisionByDigest(ctx context.Context, collectionID, digest string) (manageddata.Revision, error) {
	rows, err := a.repository.ListRevisions(ctx, collectionID)
	if err != nil {
		return manageddata.Revision{}, publicError(err)
	}
	var found *manageddata.Revision
	for i := range rows {
		if rows[i].CollectionID != collectionID {
			return manageddata.Revision{}, control.ErrBackend
		}
		if rows[i].Digest != digest || rows[i].Status != manageddata.RevisionStatusReady {
			continue
		}
		if found != nil {
			return manageddata.Revision{}, control.ErrBackend
		}
		copy := rows[i]
		found = &copy
	}
	if found == nil {
		return manageddata.Revision{}, control.ErrNotFound
	}
	return *found, nil
}

func (a *Adapter) mapRollout(ctx context.Context, row manageddata.Rollout) (control.Rollout, error) {
	revision, err := a.repository.RevisionByID(ctx, row.RevisionID)
	if err != nil {
		return control.Rollout{}, publicError(err)
	}
	if revision.CollectionID != row.CollectionID || revision.Status != manageddata.RevisionStatusReady || !canonicalRevisionID.MatchString(revision.Digest) {
		return control.Rollout{}, control.ErrBackend
	}
	status, err := publicRolloutStatus(row.Status)
	if err != nil {
		return control.Rollout{}, err
	}
	targets := append([]manageddata.RolloutTarget(nil), row.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].WorkspaceID < targets[j].WorkspaceID })
	mappedTargets := make([]control.RolloutTarget, len(targets))
	for i, target := range targets {
		targetStatus, statusErr := publicTargetStatus(target.Status, row.Status)
		if statusErr != nil {
			return control.Rollout{}, statusErr
		}
		previous, previousErr := a.previousRevisionDigest(ctx, row.CollectionID, target.PriorServingStateID)
		if previousErr != nil {
			return control.Rollout{}, previousErr
		}
		mappedTargets[i] = control.RolloutTarget{
			Workspace: target.WorkspaceID, ServingStateID: target.ServingStateID, Status: targetStatus,
			PreviousRevisionID: previous, ActivatedAt: target.ActivatedAt,
		}
		if row.Status == manageddata.RolloutStatusSuperseded {
			mappedTargets[i].RolledBackAt = row.CompletedAt
		}
	}
	mapped := control.Rollout{
		ID: row.ID, CollectionID: row.CollectionID, RevisionID: revision.Digest, Environment: string(row.Environment),
		Status: status, Targets: mappedTargets, CreatedAt: row.CreatedAt,
	}
	switch row.Status {
	case manageddata.RolloutStatusActive:
		mapped.ActivatedAt = row.CompletedAt
	case manageddata.RolloutStatusFailed:
		mapped.Error = "managed-data rollout failed"
	case manageddata.RolloutStatusSuperseded:
		mapped.ActivatedAt = row.CompletedAt
		mapped.RolledBackAt = row.CompletedAt
	}
	return mapped, nil
}

func (a *Adapter) previousRevisionDigest(ctx context.Context, collectionID, servingStateID string) (string, error) {
	if strings.TrimSpace(servingStateID) == "" {
		return "", nil
	}
	bindings, err := a.repository.ListServingStateBindings(ctx, servingStateID)
	if err != nil {
		return "", publicError(err)
	}
	for _, binding := range bindings {
		if binding.CollectionID != collectionID {
			continue
		}
		revision, revisionErr := a.repository.RevisionByID(ctx, binding.RevisionID)
		if revisionErr != nil {
			return "", publicError(revisionErr)
		}
		if revision.CollectionID != collectionID || revision.Status != manageddata.RevisionStatusReady || !canonicalRevisionID.MatchString(revision.Digest) {
			return "", control.ErrBackend
		}
		return revision.Digest, nil
	}
	return "", nil
}

func publicRolloutStatus(status manageddata.RolloutStatus) (control.RolloutStatus, error) {
	switch status {
	case manageddata.RolloutStatusPending:
		return control.RolloutStatusDraft, nil
	case manageddata.RolloutStatusActive:
		return control.RolloutStatusActive, nil
	case manageddata.RolloutStatusFailed:
		return control.RolloutStatusFailed, nil
	case manageddata.RolloutStatusSuperseded:
		return control.RolloutStatusRolledBack, nil
	default:
		return "", control.ErrBackend
	}
}

func publicTargetStatus(status manageddata.TargetStatus, rolloutStatus manageddata.RolloutStatus) (control.RolloutTargetStatus, error) {
	if rolloutStatus == manageddata.RolloutStatusSuperseded {
		return control.RolloutTargetStatusRolledBack, nil
	}
	switch status {
	case manageddata.TargetStatusPending:
		return control.RolloutTargetStatusPending, nil
	case manageddata.TargetStatusActive:
		return control.RolloutTargetStatusActive, nil
	case manageddata.TargetStatusFailed:
		return control.RolloutTargetStatusFailed, nil
	default:
		return "", control.ErrBackend
	}
}

func domainStatusFilter(status control.RolloutStatus) (manageddata.RolloutStatus, bool, error) {
	switch status {
	case "":
		return "", false, nil
	case control.RolloutStatusDraft:
		return manageddata.RolloutStatusPending, true, nil
	case control.RolloutStatusActive:
		return manageddata.RolloutStatusActive, true, nil
	case control.RolloutStatusFailed:
		return manageddata.RolloutStatusFailed, true, nil
	case control.RolloutStatusRolledBack:
		return manageddata.RolloutStatusSuperseded, true, nil
	case control.RolloutStatusActivating, control.RolloutStatusRollingBack:
		return "", true, nil
	default:
		return "", false, control.ErrInvalid
	}
}

func sameRolloutRequest(row manageddata.Rollout, collectionID, revisionID string, environment manageddata.Environment, targets []manageddata.RolloutTargetInput, actor string) bool {
	if row.CollectionID != collectionID || row.RevisionID != revisionID || row.Environment != environment || row.CreatedBy != strings.TrimSpace(actor) || len(row.Targets) != len(targets) {
		return false
	}
	actual := append([]manageddata.RolloutTarget(nil), row.Targets...)
	sort.Slice(actual, func(i, j int) bool { return actual[i].WorkspaceID < actual[j].WorkspaceID })
	for i := range targets {
		if actual[i].WorkspaceID != targets[i].WorkspaceID || actual[i].ServingStateID != targets[i].ServingStateID {
			return false
		}
	}
	return true
}

func deterministicRolloutID(project, connection, actor, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(project) + "\x00" + strings.TrimSpace(connection) + "\x00" + strings.TrimSpace(actor) + "\x00" + strings.TrimSpace(idempotencyKey)))
	return "rollout_" + hex.EncodeToString(sum[:])
}

func publicError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, manageddata.ErrNotFound), errors.Is(err, control.ErrNotFound):
		return control.ErrNotFound
	case errors.Is(err, manageddata.ErrConflict), errors.Is(err, control.ErrConflict):
		return control.ErrConflict
	case errors.Is(err, control.ErrInvalid):
		return control.ErrInvalid
	default:
		return control.ErrBackend
	}
}

var _ control.MetadataRepository = (*Adapter)(nil)
var _ control.RolloutCoordinator = (*Adapter)(nil)
