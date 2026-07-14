// Package binding pins project-global managed-data revisions to serving states.
package binding

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/manageddata"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	servingstatefs "github.com/Yacobolo/libredash/internal/servingstate/filesystem"
)

var (
	ErrArtifactMetadata          = errors.New("managed data artifact metadata is inconsistent")
	ErrPinnedRevisionUnavailable = errors.New("managed data pinned revision is unavailable")
	ErrRepository                = errors.New("managed data binding repository failure")
)

// Repository is the metadata surface needed to resolve artifact-owned pins.
// ReplaceServingStateBindings must replace the complete set atomically.
type Repository interface {
	CollectionByProjectConnection(context.Context, string, string) (manageddata.Collection, error)
	ListRevisions(context.Context, string) ([]manageddata.Revision, error)
	ReplaceServingStateBindings(context.Context, string, []manageddata.ServingStateBinding) error
}

type artifactLoader func(string) (servingstatefs.CompiledWorkspaceArtifact, error)

// Binder resolves project-global revision pins during publish validation.
type Binder struct {
	repository Repository
	load       artifactLoader
}

func New(repository Repository) (*Binder, error) {
	if repository == nil {
		return nil, fmt.Errorf("managed data binding repository is required")
	}
	return newBinder(repository, loadCompiledArtifact), nil
}

func newBinder(repository Repository, load artifactLoader) *Binder {
	return &Binder{repository: repository, load: load}
}

func loadCompiledArtifact(root string) (servingstatefs.CompiledWorkspaceArtifact, error) {
	compiled, _, err := servingstatefs.LoadCompiledWorkspaceArtifact(root)
	return compiled, err
}

// AfterArtifactValidation implements the serving-state publish validation hook.
func (b *Binder) AfterArtifactValidation(ctx context.Context, candidate servingstate.State, validation servingstate.Validation) error {
	if b == nil || b.repository == nil || b.load == nil {
		return ErrRepository
	}
	if strings.TrimSpace(string(candidate.ID)) == "" || strings.TrimSpace(validation.RootDir) == "" {
		return ErrArtifactMetadata
	}
	compiled, err := b.load(validation.RootDir)
	if err != nil {
		return ErrArtifactMetadata
	}
	if err := servingstatefs.ValidateCompiledWorkspaceArtifact(compiled); err != nil {
		return ErrArtifactMetadata
	}

	environment, err := manageddata.NormalizeEnvironment(string(servingstate.NormalizeEnvironment(candidate.Environment)))
	if err != nil {
		return ErrArtifactMetadata
	}
	connections := make([]string, 0, len(compiled.ManagedDataRevisions))
	for connection := range compiled.ManagedDataRevisions {
		connections = append(connections, connection)
	}
	sort.Strings(connections)
	bindings := make([]manageddata.ServingStateBinding, 0, len(connections))
	for _, connection := range connections {
		binding, bindErr := b.pinnedBinding(ctx, candidate.ID, compiled.ProjectID, connection, compiled.ManagedDataRevisions[connection], environment)
		if bindErr != nil {
			return bindErr
		}
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].CollectionID != bindings[j].CollectionID {
			return bindings[i].CollectionID < bindings[j].CollectionID
		}
		return bindings[i].RevisionID < bindings[j].RevisionID
	})
	if err := b.repository.ReplaceServingStateBindings(ctx, string(candidate.ID), bindings); err != nil {
		return repositoryError(err)
	}
	return nil
}

func (b *Binder) pinnedBinding(ctx context.Context, servingStateID servingstate.ID, projectID, connectionName, digest string, environment manageddata.Environment) (manageddata.ServingStateBinding, error) {
	collection, err := b.repository.CollectionByProjectConnection(ctx, projectID, connectionName)
	if err != nil {
		if errors.Is(err, manageddata.ErrNotFound) {
			return manageddata.ServingStateBinding{}, ErrPinnedRevisionUnavailable
		}
		return manageddata.ServingStateBinding{}, repositoryError(err)
	}
	if collection.ID == "" || collection.ProjectID != projectID || collection.ConnectionName != connectionName {
		return manageddata.ServingStateBinding{}, ErrArtifactMetadata
	}
	if collection.Status != manageddata.CollectionStatusActive {
		return manageddata.ServingStateBinding{}, ErrPinnedRevisionUnavailable
	}

	revisions, err := b.repository.ListRevisions(ctx, collection.ID)
	if err != nil {
		return manageddata.ServingStateBinding{}, repositoryError(err)
	}
	var match manageddata.Revision
	matches := 0
	for _, revision := range revisions {
		if revision.CollectionID != collection.ID {
			return manageddata.ServingStateBinding{}, ErrArtifactMetadata
		}
		if revision.Digest == digest && revision.Status == manageddata.RevisionStatusReady {
			match = revision
			matches++
		}
	}
	if matches > 1 {
		return manageddata.ServingStateBinding{}, ErrArtifactMetadata
	}
	if matches == 0 {
		return manageddata.ServingStateBinding{}, ErrPinnedRevisionUnavailable
	}
	return manageddata.ServingStateBinding{
		ServingStateID: string(servingStateID),
		CollectionID:   collection.ID,
		RevisionID:     match.ID,
		Environment:    environment,
	}, nil
}

func repositoryError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ErrRepository
}
