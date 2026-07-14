package sqlite

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/manageddata"
	platformdb "github.com/Yacobolo/libredash/internal/platform/db"
)

type Repository struct {
	db *sql.DB
	q  *platformdb.Queries
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db, q: platformdb.New(db)} }

func (r *Repository) CreateCollection(ctx context.Context, input manageddata.CreateCollectionInput) (manageddata.Collection, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.ConnectionName = strings.TrimSpace(input.ConnectionName)
	input.Name = strings.TrimSpace(input.Name)
	if input.ID != "" {
		if err := manageddata.ValidateCollectionID(input.ID); err != nil {
			return manageddata.Collection{}, err
		}
	}
	if err := validateIdentityPart("project id", input.ProjectID); err != nil {
		return manageddata.Collection{}, err
	}
	if err := validateIdentityPart("connection name", input.ConnectionName); err != nil {
		return manageddata.Collection{}, err
	}
	if input.Name == "" {
		input.Name = input.ConnectionName
	}
	if existing, err := r.CollectionByProjectConnection(ctx, input.ProjectID, input.ConnectionName); err == nil {
		return idempotentCollection(existing, input)
	} else if !errors.Is(err, manageddata.ErrNotFound) {
		return manageddata.Collection{}, err
	}
	var err error
	if input.ID == "" {
		input.ID, err = newID("collection")
		if err != nil {
			return manageddata.Collection{}, err
		}
	}
	err = r.q.CreateManagedDataCollection(ctx, platformdb.CreateManagedDataCollectionParams{
		ID: input.ID, ProjectID: input.ProjectID, ConnectionName: input.ConnectionName,
		Name: input.Name, Description: strings.TrimSpace(input.Description), CreatedBy: strings.TrimSpace(input.CreatedBy),
	})
	if err != nil {
		if existing, lookupErr := r.CollectionByProjectConnection(ctx, input.ProjectID, input.ConnectionName); lookupErr == nil {
			return idempotentCollection(existing, input)
		}
		return manageddata.Collection{}, mapError(err)
	}
	return r.CollectionByID(ctx, input.ID)
}

func (r *Repository) CollectionByProjectConnection(ctx context.Context, projectID, connectionName string) (manageddata.Collection, error) {
	row, err := r.q.GetManagedDataCollectionByProjectConnection(ctx, platformdb.GetManagedDataCollectionByProjectConnectionParams{
		ProjectID: strings.TrimSpace(projectID), ConnectionName: strings.TrimSpace(connectionName),
	})
	if err != nil {
		return manageddata.Collection{}, mapError(err)
	}
	return mapCollection(row), nil
}

func (r *Repository) CollectionByID(ctx context.Context, id string) (manageddata.Collection, error) {
	row, err := r.q.GetManagedDataCollection(ctx, strings.TrimSpace(id))
	if err != nil {
		return manageddata.Collection{}, mapError(err)
	}
	return mapCollection(row), nil
}

func (r *Repository) ListCollections(ctx context.Context, includeArchived bool) ([]manageddata.Collection, error) {
	var rows []platformdb.ManagedDataCollection
	var err error
	if includeArchived {
		rows, err = r.q.ListAllManagedDataCollections(ctx)
	} else {
		rows, err = r.q.ListActiveManagedDataCollections(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]manageddata.Collection, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCollection(row))
	}
	return out, nil
}

func (r *Repository) ArchiveCollection(ctx context.Context, id string) error {
	result, err := r.q.ArchiveManagedDataCollection(ctx, strings.TrimSpace(id))
	return expectOne(result, err, "collection is not active")
}

func (r *Repository) CreateUploadSession(ctx context.Context, input manageddata.CreateUploadSessionInput) (manageddata.UploadSession, error) {
	input.CollectionID = strings.TrimSpace(input.CollectionID)
	input.BaseRevisionID = strings.TrimSpace(input.BaseRevisionID)
	input.StorageBackend = strings.TrimSpace(input.StorageBackend)
	input.StagingPrefix = strings.TrimSpace(input.StagingPrefix)
	if input.CollectionID == "" {
		return manageddata.UploadSession{}, fmt.Errorf("collection id is required")
	}
	if input.StorageBackend == "" {
		return manageddata.UploadSession{}, fmt.Errorf("storage backend is required")
	}
	if input.StagingPrefix == "" {
		return manageddata.UploadSession{}, fmt.Errorf("staging prefix is required")
	}
	if input.ExpiresAt.IsZero() || !input.ExpiresAt.After(time.Now()) {
		return manageddata.UploadSession{}, fmt.Errorf("upload session expiry must be in the future")
	}
	manifestJSON, err := input.Manifest.CanonicalJSON()
	if err != nil {
		return manageddata.UploadSession{}, err
	}
	fileCount, sizeBytes := manifestTotals(input.Manifest)
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id, err = newID("upload")
		if err != nil {
			return manageddata.UploadSession{}, err
		}
	}
	err = r.q.CreateManagedDataUploadSession(ctx, platformdb.CreateManagedDataUploadSessionParams{
		ID: id, CollectionID: input.CollectionID, BaseRevisionID: nullable(input.BaseRevisionID), ManifestJson: string(manifestJSON),
		ExpectedFileCount: fileCount, ExpectedSizeBytes: sizeBytes, StorageBackend: input.StorageBackend,
		StagingPrefix: input.StagingPrefix, CreatedBy: strings.TrimSpace(input.CreatedBy), ExpiresAt: timestamp(input.ExpiresAt),
	})
	if err != nil {
		return manageddata.UploadSession{}, mapError(err)
	}
	return r.UploadSessionByID(ctx, id)
}

func (r *Repository) UploadSessionByID(ctx context.Context, id string) (manageddata.UploadSession, error) {
	row, err := r.q.GetManagedDataUploadSession(ctx, strings.TrimSpace(id))
	if err != nil {
		return manageddata.UploadSession{}, mapError(err)
	}
	return mapUploadSession(row), nil
}

func (r *Repository) UpdateUploadProgress(ctx context.Context, id string, progress manageddata.UploadProgress) error {
	if progress.UploadedFileCount < 0 || progress.UploadedSizeBytes < 0 {
		return fmt.Errorf("upload progress cannot be negative")
	}
	result, err := r.q.UpdateManagedDataUploadProgress(ctx, platformdb.UpdateManagedDataUploadProgressParams{
		UploadedFileCount: progress.UploadedFileCount, UploadedSizeBytes: progress.UploadedSizeBytes, ID: strings.TrimSpace(id),
		ExpectedFileCount: progress.UploadedFileCount, ExpectedSizeBytes: progress.UploadedSizeBytes,
	})
	return expectOne(result, err, "upload session is not open or progress exceeds its manifest")
}

func (r *Repository) AbortUploadSession(ctx context.Context, id string) error {
	result, err := r.q.AbortManagedDataUploadSession(ctx, strings.TrimSpace(id))
	return expectOne(result, err, "upload session is not open")
}

func (r *Repository) ExpireUploadSessions(ctx context.Context, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now()
	}
	result, err := r.q.ExpireManagedDataUploadSessions(ctx, timestamp(now))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Repository) CompleteUpload(ctx context.Context, input manageddata.CompleteUploadInput) (manageddata.Revision, error) {
	input.SessionID = strings.TrimSpace(input.SessionID)
	if input.SessionID == "" {
		return manageddata.Revision{}, fmt.Errorf("upload session id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return manageddata.Revision{}, err
	}
	defer tx.Rollback()
	q := r.q.WithTx(tx)
	result, err := q.MarkManagedDataUploadCommitting(ctx, input.SessionID)
	if err != nil {
		return manageddata.Revision{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return manageddata.Revision{}, err
	}
	if affected != 1 {
		session, getErr := q.GetManagedDataUploadSession(ctx, input.SessionID)
		if getErr != nil {
			return manageddata.Revision{}, mapError(getErr)
		}
		if session.Status == string(manageddata.UploadStatusComplete) && session.RevisionID.Valid {
			row, getErr := q.GetManagedDataRevision(ctx, session.RevisionID.String)
			return mapRevision(row), mapError(getErr)
		}
		return manageddata.Revision{}, fmt.Errorf("%w: upload session is %s or expired", manageddata.ErrConflict, session.Status)
	}
	session, err := q.GetManagedDataUploadSession(ctx, input.SessionID)
	if err != nil {
		return manageddata.Revision{}, err
	}
	manifest, err := decodeManifest(session.ManifestJson)
	if err != nil {
		return manageddata.Revision{}, err
	}
	if err := validateStoredFiles(manifest, input.Files); err != nil {
		return manageddata.Revision{}, err
	}
	sequence, err := q.NextManagedDataRevisionSequence(ctx, session.CollectionID)
	if err != nil {
		return manageddata.Revision{}, err
	}
	revisionID := strings.TrimSpace(input.RevisionID)
	if revisionID == "" {
		revisionID, err = newID("revision")
		if err != nil {
			return manageddata.Revision{}, err
		}
	}
	if err := q.CreateReadyManagedDataRevision(ctx, platformdb.CreateReadyManagedDataRevisionParams{
		ID: revisionID, CollectionID: session.CollectionID, Sequence: sequence, Digest: manifest.RevisionID(),
		ManifestJson: session.ManifestJson, FileCount: session.ExpectedFileCount, SizeBytes: session.ExpectedSizeBytes, CreatedBy: session.CreatedBy,
	}); err != nil {
		return manageddata.Revision{}, mapError(err)
	}
	for _, file := range sortedStoredFiles(input.Files) {
		if err := q.CreateManagedDataRevisionFile(ctx, platformdb.CreateManagedDataRevisionFileParams{
			RevisionID: revisionID, LogicalPath: file.Path, SizeBytes: file.Size, Sha256: file.SHA256,
			StorageKey: file.StorageKey, MediaType: strings.TrimSpace(file.MediaType), Etag: strings.TrimSpace(file.ETag),
		}); err != nil {
			return manageddata.Revision{}, mapError(err)
		}
	}
	result, err = q.CompleteManagedDataUploadSession(ctx, platformdb.CompleteManagedDataUploadSessionParams{RevisionID: nullable(revisionID), ID: input.SessionID})
	if err != nil {
		return manageddata.Revision{}, err
	}
	if err := requireOne(result, "upload session changed while committing"); err != nil {
		return manageddata.Revision{}, err
	}
	if err := tx.Commit(); err != nil {
		return manageddata.Revision{}, mapError(err)
	}
	return r.RevisionByID(ctx, revisionID)
}

func (r *Repository) RevisionByID(ctx context.Context, id string) (manageddata.Revision, error) {
	row, err := r.q.GetManagedDataRevision(ctx, strings.TrimSpace(id))
	if err != nil {
		return manageddata.Revision{}, mapError(err)
	}
	return mapRevision(row), nil
}

func (r *Repository) ListRevisions(ctx context.Context, collectionID string) ([]manageddata.Revision, error) {
	rows, err := r.q.ListManagedDataRevisions(ctx, strings.TrimSpace(collectionID))
	if err != nil {
		return nil, err
	}
	out := make([]manageddata.Revision, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRevision(row))
	}
	return out, nil
}

func (r *Repository) ListRevisionFiles(ctx context.Context, revisionID string) ([]manageddata.RevisionFile, error) {
	rows, err := r.q.ListManagedDataRevisionFiles(ctx, strings.TrimSpace(revisionID))
	if err != nil {
		return nil, err
	}
	out := make([]manageddata.RevisionFile, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRevisionFile(row))
	}
	return out, nil
}

func (r *Repository) CreateRollout(ctx context.Context, input manageddata.CreateRolloutInput) (manageddata.Rollout, error) {
	input.CollectionID = strings.TrimSpace(input.CollectionID)
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	environment, err := manageddata.NormalizeEnvironment(string(input.Environment))
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if len(input.Targets) == 0 {
		return manageddata.Rollout{}, fmt.Errorf("rollout requires at least one workspace target")
	}
	revision, err := r.RevisionByID(ctx, input.RevisionID)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if revision.CollectionID != input.CollectionID || revision.Status != manageddata.RevisionStatusReady {
		return manageddata.Rollout{}, fmt.Errorf("revision %q is not a ready revision of collection %q", input.RevisionID, input.CollectionID)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id, err = newID("rollout")
		if err != nil {
			return manageddata.Rollout{}, err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	defer tx.Rollback()
	q := r.q.WithTx(tx)
	if err := q.CreateManagedDataRollout(ctx, platformdb.CreateManagedDataRolloutParams{
		ID: id, CollectionID: input.CollectionID, Environment: string(environment), RevisionID: input.RevisionID, CreatedBy: strings.TrimSpace(input.CreatedBy),
	}); err != nil {
		return manageddata.Rollout{}, mapError(err)
	}
	targets := append([]manageddata.RolloutTargetInput(nil), input.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].WorkspaceID < targets[j].WorkspaceID })
	seen := map[string]struct{}{}
	for _, target := range targets {
		target.WorkspaceID = strings.TrimSpace(target.WorkspaceID)
		target.ServingStateID = strings.TrimSpace(target.ServingStateID)
		if target.WorkspaceID == "" || target.ServingStateID == "" {
			return manageddata.Rollout{}, fmt.Errorf("rollout target workspace and serving state ids are required")
		}
		if _, exists := seen[target.WorkspaceID]; exists {
			return manageddata.Rollout{}, fmt.Errorf("duplicate rollout workspace %q", target.WorkspaceID)
		}
		seen[target.WorkspaceID] = struct{}{}
		candidate, err := q.GetServingState(ctx, target.ServingStateID)
		if err != nil {
			return manageddata.Rollout{}, mapError(err)
		}
		if candidate.WorkspaceID != target.WorkspaceID || candidate.Environment != string(environment) || candidate.Status != "validated" {
			return manageddata.Rollout{}, fmt.Errorf("%w: serving state %q is not a validated %s candidate for workspace %q", manageddata.ErrConflict, target.ServingStateID, environment, target.WorkspaceID)
		}
		priorID, err := q.GetWorkspaceActiveServingStateID(ctx, platformdb.GetWorkspaceActiveServingStateIDParams{WorkspaceID: target.WorkspaceID, Environment: string(environment)})
		if errors.Is(err, sql.ErrNoRows) {
			priorID = ""
		} else if err != nil {
			return manageddata.Rollout{}, err
		}
		if err := q.CreateManagedDataRolloutTarget(ctx, platformdb.CreateManagedDataRolloutTargetParams{
			RolloutID: id, WorkspaceID: target.WorkspaceID, ServingStateID: target.ServingStateID, PriorServingStateID: nullable(priorID),
		}); err != nil {
			return manageddata.Rollout{}, mapError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return manageddata.Rollout{}, err
	}
	return r.RolloutByID(ctx, id)
}

func (r *Repository) RolloutByID(ctx context.Context, id string) (manageddata.Rollout, error) {
	row, err := r.q.GetManagedDataRollout(ctx, strings.TrimSpace(id))
	if err != nil {
		return manageddata.Rollout{}, mapError(err)
	}
	targetRows, err := r.q.ListManagedDataRolloutTargets(ctx, row.ID)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	return mapRollout(row, targetRows), nil
}

func (r *Repository) ActivateRollout(ctx context.Context, id string, expected manageddata.PointerExpectation) (manageddata.Rollout, error) {
	id = strings.TrimSpace(id)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	defer tx.Rollback()
	q := r.q.WithTx(tx)
	rollout, err := q.GetManagedDataRollout(ctx, id)
	if err != nil {
		return manageddata.Rollout{}, mapError(err)
	}
	if rollout.Status != string(manageddata.RolloutStatusPending) {
		return manageddata.Rollout{}, fmt.Errorf("%w: rollout is %s", manageddata.ErrConflict, rollout.Status)
	}
	pointer, getErr := q.GetManagedDataEnvironmentPointer(ctx, platformdb.GetManagedDataEnvironmentPointerParams{CollectionID: rollout.CollectionID, Environment: rollout.Environment})
	if errors.Is(getErr, sql.ErrNoRows) {
		if expected.Generation != 0 || expected.RevisionID != "" {
			return manageddata.Rollout{}, fmt.Errorf("%w: collection environment has no current revision", manageddata.ErrConflict)
		}
		pointer = platformdb.ManagedDataEnvironmentPointer{CollectionID: rollout.CollectionID, Environment: rollout.Environment}
	} else if getErr != nil {
		return manageddata.Rollout{}, getErr
	} else if pointer.Generation != expected.Generation || pointer.RevisionID != expected.RevisionID {
		return manageddata.Rollout{}, fmt.Errorf("%w: collection environment pointer changed", manageddata.ErrConflict)
	}
	targets, err := q.ListManagedDataRolloutTargets(ctx, id)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	for _, target := range targets {
		candidate, err := q.GetServingState(ctx, target.ServingStateID)
		if err != nil {
			return manageddata.Rollout{}, mapError(err)
		}
		if candidate.WorkspaceID != target.WorkspaceID || candidate.Environment != rollout.Environment || candidate.Status != "validated" {
			return manageddata.Rollout{}, fmt.Errorf("%w: rollout target %q is no longer a validated candidate", manageddata.ErrConflict, target.ServingStateID)
		}
		activeID, err := q.GetWorkspaceActiveServingStateID(ctx, platformdb.GetWorkspaceActiveServingStateIDParams{WorkspaceID: target.WorkspaceID, Environment: rollout.Environment})
		if errors.Is(err, sql.ErrNoRows) {
			activeID = ""
		} else if err != nil {
			return manageddata.Rollout{}, err
		}
		if activeID != target.PriorServingStateID.String {
			return manageddata.Rollout{}, fmt.Errorf("%w: workspace %q active serving state changed", manageddata.ErrConflict, target.WorkspaceID)
		}
	}
	generation := pointer.Generation + 1
	if err := q.UpsertManagedDataEnvironmentPointer(ctx, platformdb.UpsertManagedDataEnvironmentPointerParams{
		CollectionID: rollout.CollectionID, Environment: rollout.Environment, RevisionID: rollout.RevisionID,
		RolloutID: rollout.ID, Generation: generation, UpdatedBy: rollout.CreatedBy,
	}); err != nil {
		return manageddata.Rollout{}, err
	}
	for _, target := range targets {
		if err := q.MarkOtherServingStatesDraining(ctx, platformdb.MarkOtherServingStatesDrainingParams{WorkspaceID: target.WorkspaceID, Environment: rollout.Environment, ID: target.ServingStateID}); err != nil {
			return manageddata.Rollout{}, err
		}
		if err := q.MarkServingStateActive(ctx, target.ServingStateID); err != nil {
			return manageddata.Rollout{}, err
		}
		if err := q.SetActiveServingState(ctx, platformdb.SetActiveServingStateParams{WorkspaceID: target.WorkspaceID, Environment: rollout.Environment, ServingStateID: target.ServingStateID}); err != nil {
			return manageddata.Rollout{}, err
		}
		if err := q.CreateManagedDataServingStateBinding(ctx, platformdb.CreateManagedDataServingStateBindingParams{ServingStateID: target.ServingStateID, CollectionID: rollout.CollectionID, RevisionID: rollout.RevisionID, Environment: rollout.Environment}); err != nil {
			return manageddata.Rollout{}, err
		}
		if err := q.ActivateManagedDataRolloutTarget(ctx, platformdb.ActivateManagedDataRolloutTargetParams{RolloutID: rollout.ID, WorkspaceID: target.WorkspaceID}); err != nil {
			return manageddata.Rollout{}, err
		}
	}
	result, err := q.ActivateManagedDataRollout(ctx, rollout.ID)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if err := requireOne(result, "rollout changed while activating"); err != nil {
		return manageddata.Rollout{}, err
	}
	if pointer.RolloutID != "" && pointer.RolloutID != rollout.ID {
		if err := q.SupersedeManagedDataRollout(ctx, pointer.RolloutID); err != nil {
			return manageddata.Rollout{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return manageddata.Rollout{}, mapError(err)
	}
	return r.RolloutByID(ctx, id)
}

func (r *Repository) FailRollout(ctx context.Context, id string, cause error) error {
	if cause == nil || strings.TrimSpace(cause.Error()) == "" {
		return fmt.Errorf("rollout failure cause is required")
	}
	result, err := r.q.FailManagedDataRollout(ctx, platformdb.FailManagedDataRolloutParams{Error: cause.Error(), ID: strings.TrimSpace(id)})
	return expectOne(result, err, "rollout is not pending")
}

func (r *Repository) EnvironmentPointer(ctx context.Context, collectionID string, environment manageddata.Environment) (manageddata.EnvironmentPointer, error) {
	normalized, err := manageddata.NormalizeEnvironment(string(environment))
	if err != nil {
		return manageddata.EnvironmentPointer{}, err
	}
	row, err := r.q.GetManagedDataEnvironmentPointer(ctx, platformdb.GetManagedDataEnvironmentPointerParams{CollectionID: strings.TrimSpace(collectionID), Environment: string(normalized)})
	if err != nil {
		return manageddata.EnvironmentPointer{}, mapError(err)
	}
	return mapEnvironmentPointer(row), nil
}

func (r *Repository) ReplaceServingStateBindings(ctx context.Context, servingStateID string, bindings []manageddata.ServingStateBinding) error {
	servingStateID = strings.TrimSpace(servingStateID)
	if servingStateID == "" {
		return fmt.Errorf("serving state id is required")
	}
	normalized := make([]manageddata.ServingStateBinding, 0, len(bindings))
	seen := map[string]struct{}{}
	for _, binding := range bindings {
		binding.CollectionID = strings.TrimSpace(binding.CollectionID)
		binding.RevisionID = strings.TrimSpace(binding.RevisionID)
		if binding.CollectionID == "" || binding.RevisionID == "" {
			return fmt.Errorf("binding collection and revision ids are required")
		}
		if _, exists := seen[binding.CollectionID]; exists {
			return fmt.Errorf("duplicate binding for collection %q", binding.CollectionID)
		}
		seen[binding.CollectionID] = struct{}{}
		environment, err := manageddata.NormalizeEnvironment(string(binding.Environment))
		if err != nil {
			return err
		}
		binding.Environment = environment
		binding.ServingStateID = servingStateID
		normalized = append(normalized, binding)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].CollectionID < normalized[j].CollectionID })
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := r.q.WithTx(tx)
	if err := q.DeleteManagedDataServingStateBindings(ctx, servingStateID); err != nil {
		return err
	}
	for _, binding := range normalized {
		if err := q.CreateManagedDataServingStateBinding(ctx, platformdb.CreateManagedDataServingStateBindingParams{
			ServingStateID: servingStateID, CollectionID: binding.CollectionID, RevisionID: binding.RevisionID, Environment: string(binding.Environment),
		}); err != nil {
			return mapError(err)
		}
	}
	return tx.Commit()
}

func (r *Repository) ListServingStateBindings(ctx context.Context, servingStateID string) ([]manageddata.ServingStateBinding, error) {
	rows, err := r.q.ListManagedDataServingStateBindings(ctx, strings.TrimSpace(servingStateID))
	if err != nil {
		return nil, err
	}
	out := make([]manageddata.ServingStateBinding, 0, len(rows))
	for _, row := range rows {
		out = append(out, manageddata.ServingStateBinding{ServingStateID: row.ServingStateID, CollectionID: row.CollectionID, RevisionID: row.RevisionID, Environment: manageddata.Environment(row.Environment), BoundAt: row.BoundAt})
	}
	return out, nil
}

func validateStoredFiles(manifest manageddata.Manifest, files []manageddata.StoredFile) error {
	if len(files) != len(manifest.Files) {
		return fmt.Errorf("stored file count %d does not match manifest count %d", len(files), len(manifest.Files))
	}
	actual := manageddata.Manifest{Files: make([]manageddata.File, 0, len(files))}
	for _, file := range files {
		if strings.TrimSpace(file.StorageKey) == "" {
			return fmt.Errorf("stored file %q has no storage key", file.Path)
		}
		actual.Files = append(actual.Files, file.File)
	}
	wantJSON, err := manifest.CanonicalJSON()
	if err != nil {
		return err
	}
	actualJSON, err := actual.CanonicalJSON()
	if err != nil {
		return err
	}
	if !bytes.Equal(wantJSON, actualJSON) {
		return fmt.Errorf("stored files do not match upload manifest")
	}
	return nil
}

func decodeManifest(value string) (manageddata.Manifest, error) {
	var manifest manageddata.Manifest
	if err := json.Unmarshal([]byte(value), &manifest); err != nil {
		return manageddata.Manifest{}, fmt.Errorf("decode upload manifest: %w", err)
	}
	if err := manifest.Validate(manageddata.Limits{}); err != nil {
		return manageddata.Manifest{}, err
	}
	return manifest, nil
}

func manifestTotals(manifest manageddata.Manifest) (int64, int64) {
	var size int64
	for _, file := range manifest.Files {
		size += file.Size
	}
	return int64(len(manifest.Files)), size
}

func sortedStoredFiles(files []manageddata.StoredFile) []manageddata.StoredFile {
	out := append([]manageddata.StoredFile(nil), files...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func mapCollection(row platformdb.ManagedDataCollection) manageddata.Collection {
	return manageddata.Collection{ID: row.ID, ProjectID: row.ProjectID, ConnectionName: row.ConnectionName, Name: row.Name, Description: row.Description, Status: manageddata.CollectionStatus(row.Status), CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ArchivedAt: row.ArchivedAt.String}
}

func mapRevision(row platformdb.ManagedDataRevision) manageddata.Revision {
	return manageddata.Revision{ID: row.ID, CollectionID: row.CollectionID, Sequence: row.Sequence, Digest: row.Digest, Status: manageddata.RevisionStatus(row.Status), ManifestJSON: row.ManifestJson, FileCount: row.FileCount, SizeBytes: row.SizeBytes, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, ReadyAt: row.ReadyAt.String, Error: row.Error}
}

func mapRevisionFile(row platformdb.ManagedDataRevisionFile) manageddata.RevisionFile {
	return manageddata.RevisionFile{RevisionID: row.RevisionID, StoredFile: manageddata.StoredFile{File: manageddata.File{Path: row.LogicalPath, Size: row.SizeBytes, SHA256: row.Sha256}, StorageKey: row.StorageKey, MediaType: row.MediaType, ETag: row.Etag}, CreatedAt: row.CreatedAt}
}

func mapUploadSession(row platformdb.ManagedDataUploadSession) manageddata.UploadSession {
	return manageddata.UploadSession{ID: row.ID, CollectionID: row.CollectionID, BaseRevisionID: row.BaseRevisionID.String, RevisionID: row.RevisionID.String, Status: manageddata.UploadStatus(row.Status), ManifestJSON: row.ManifestJson, ExpectedFileCount: row.ExpectedFileCount, ExpectedSizeBytes: row.ExpectedSizeBytes, UploadedFileCount: row.UploadedFileCount, UploadedSizeBytes: row.UploadedSizeBytes, StorageBackend: row.StorageBackend, StagingPrefix: row.StagingPrefix, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ExpiresAt: row.ExpiresAt, CompletedAt: row.CompletedAt.String, Error: row.Error}
}

func mapRollout(row platformdb.ManagedDataRollout, targetRows []platformdb.ManagedDataRolloutTarget) manageddata.Rollout {
	targets := make([]manageddata.RolloutTarget, 0, len(targetRows))
	for _, target := range targetRows {
		targets = append(targets, manageddata.RolloutTarget{RolloutID: target.RolloutID, WorkspaceID: target.WorkspaceID, ServingStateID: target.ServingStateID, PriorServingStateID: target.PriorServingStateID.String, Status: manageddata.TargetStatus(target.Status), ActivatedAt: target.ActivatedAt.String, Error: target.Error})
	}
	return manageddata.Rollout{ID: row.ID, CollectionID: row.CollectionID, Environment: manageddata.Environment(row.Environment), RevisionID: row.RevisionID, Status: manageddata.RolloutStatus(row.Status), CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt.String, Error: row.Error, Targets: targets}
}

func validateIdentityPart(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > 255 {
		return fmt.Errorf("%s exceeds 255 characters", name)
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("%s contains control characters", name)
		}
	}
	return nil
}

func idempotentCollection(existing manageddata.Collection, input manageddata.CreateCollectionInput) (manageddata.Collection, error) {
	if input.ID != "" && existing.ID != input.ID || existing.Name != input.Name || existing.Description != strings.TrimSpace(input.Description) {
		return manageddata.Collection{}, fmt.Errorf("%w: collection %q/%q already exists with different identity or metadata", manageddata.ErrConflict, input.ProjectID, input.ConnectionName)
	}
	return existing, nil
}

func mapEnvironmentPointer(row platformdb.ManagedDataEnvironmentPointer) manageddata.EnvironmentPointer {
	return manageddata.EnvironmentPointer{CollectionID: row.CollectionID, Environment: manageddata.Environment(row.Environment), RevisionID: row.RevisionID, RolloutID: row.RolloutID, Generation: row.Generation, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt}
}

func timestamp(value time.Time) string { return value.UTC().Format("2006-01-02 15:04:05.000000000") }

func nullable(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func newID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(raw[:]), nil
}

func expectOne(result sql.Result, err error, message string) error {
	if err != nil {
		return mapError(err)
	}
	return requireOne(result, message)
}

func requireOne(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s", manageddata.ErrConflict, message)
	}
	return nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return manageddata.ErrNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "foreign key constraint") || strings.Contains(message, "constraint failed") {
		return fmt.Errorf("%w: %v", manageddata.ErrConflict, err)
	}
	return err
}

var _ manageddata.Repository = (*Repository)(nil)
