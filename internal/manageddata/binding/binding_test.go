package binding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/manageddata"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	servingstatefs "github.com/Yacobolo/libredash/internal/servingstate/filesystem"
	"github.com/Yacobolo/libredash/internal/workspace"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestBinderResolvesArtifactPinsWithinEachCollection(t *testing.T) {
	repo := &fakeRepository{
		collections: map[string]manageddata.Collection{
			"project-a\x00orders":    activeCollection("collection-z", "orders"),
			"project-a\x00customers": activeCollection("collection-a", "customers"),
		},
		revisions: map[string][]manageddata.Revision{
			"collection-z": {{ID: "orders-r2", CollectionID: "collection-z", Digest: digestA, Status: manageddata.RevisionStatusReady}},
			// The same content digest in another collection must resolve to that collection's internal revision.
			"collection-a": {{ID: "customers-r4", CollectionID: "collection-a", Digest: digestA, Status: manageddata.RevisionStatusReady}},
		},
	}
	artifact := compiledArtifact("project-a", map[string]map[string]string{
		"sales":   {"orders": "managed", "local": "local"},
		"service": {"customers": "managed", "orders": "managed"},
	}, map[string]string{"customers": digestA, "orders": digestA})
	binder := newBinder(repo, func(string) (servingstatefs.CompiledWorkspaceArtifact, error) { return artifact, nil })

	err := binder.AfterArtifactValidation(t.Context(), servingstate.State{
		ID: "state-1", WorkspaceID: "sales", Environment: "prod",
	}, servingstate.Validation{RootDir: "/validated/artifact"})
	if err != nil {
		t.Fatalf("AfterArtifactValidation() error = %v", err)
	}
	want := []manageddata.ServingStateBinding{
		{ServingStateID: "state-1", CollectionID: "collection-a", RevisionID: "customers-r4", Environment: "prod"},
		{ServingStateID: "state-1", CollectionID: "collection-z", RevisionID: "orders-r2", Environment: "prod"},
	}
	if !reflect.DeepEqual(repo.replaced, want) {
		t.Fatalf("bindings = %#v, want %#v", repo.replaced, want)
	}
	if got, want := repo.collectionLookups, []string{"project-a\x00customers", "project-a\x00orders"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collection lookups = %#v, want %#v", got, want)
	}
}

func TestBinderCanPinBootstrapRevisionWithoutEnvironmentPointer(t *testing.T) {
	repo := validFakeRepository()
	binder := binderForRepository(repo)
	if err := binder.AfterArtifactValidation(t.Context(), servingstate.State{ID: "bootstrap", Environment: "prod"}, servingstate.Validation{RootDir: "/artifact"}); err != nil {
		t.Fatalf("AfterArtifactValidation() error = %v", err)
	}
	if repo.listCalls != 1 || len(repo.replaced) != 1 || repo.replaced[0].RevisionID != "revision-1" {
		t.Fatalf("repository result = calls %d bindings %#v", repo.listCalls, repo.replaced)
	}
}

func TestBinderReplacesFullBindingSetForArtifactWithoutManagedConnections(t *testing.T) {
	repo := &fakeRepository{replaced: []manageddata.ServingStateBinding{{CollectionID: "stale"}}}
	artifact := compiledArtifact("project-a", map[string]map[string]string{"sales": {"local": "local"}}, map[string]string{})
	binder := newBinder(repo, func(string) (servingstatefs.CompiledWorkspaceArtifact, error) { return artifact, nil })
	if err := binder.AfterArtifactValidation(t.Context(), servingstate.State{ID: "state-1", Environment: "dev"}, servingstate.Validation{RootDir: "/artifact"}); err != nil {
		t.Fatal(err)
	}
	if repo.replaceCalls != 1 || len(repo.replaced) != 0 {
		t.Fatalf("replace calls = %d, bindings = %#v", repo.replaceCalls, repo.replaced)
	}
}

func TestBinderRejectsInvalidOrUnavailablePinsBeforeAtomicReplacement(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeRepository, *servingstatefs.CompiledWorkspaceArtifact)
		want   error
	}{
		{name: "missing collection", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			delete(repo.collections, "project-a\x00orders")
		}, want: ErrPinnedRevisionUnavailable},
		{name: "archived collection", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			collection := repo.collections["project-a\x00orders"]
			collection.Status = manageddata.CollectionStatusArchived
			repo.collections["project-a\x00orders"] = collection
		}, want: ErrPinnedRevisionUnavailable},
		{name: "missing ready revision", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			repo.revisions["orders"] = nil
		}, want: ErrPinnedRevisionUnavailable},
		{name: "pending revision", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			repo.revisions["orders"][0].Status = manageddata.RevisionStatusPending
		}, want: ErrPinnedRevisionUnavailable},
		{name: "ambiguous revision", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			repo.revisions["orders"] = append(repo.revisions["orders"], repo.revisions["orders"][0])
		}, want: ErrArtifactMetadata},
		{name: "wrong collection revision", mutate: func(repo *fakeRepository, _ *servingstatefs.CompiledWorkspaceArtifact) {
			repo.revisions["orders"][0].CollectionID = "other"
		}, want: ErrArtifactMetadata},
		{name: "missing pin", mutate: func(_ *fakeRepository, artifact *servingstatefs.CompiledWorkspaceArtifact) {
			delete(artifact.ManagedDataRevisions, "orders")
		}, want: ErrArtifactMetadata},
		{name: "internal id pin", mutate: func(_ *fakeRepository, artifact *servingstatefs.CompiledWorkspaceArtifact) {
			artifact.ManagedDataRevisions["orders"] = "revision-1"
		}, want: ErrArtifactMetadata},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := validFakeRepository()
			artifact := compiledArtifact("project-a", map[string]map[string]string{"sales": {"orders": "managed"}}, map[string]string{"orders": digestA})
			test.mutate(repo, &artifact)
			binder := newBinder(repo, func(string) (servingstatefs.CompiledWorkspaceArtifact, error) { return artifact, nil })
			err := binder.AfterArtifactValidation(t.Context(), servingstate.State{ID: "state-1", Environment: "prod"}, servingstate.Validation{RootDir: "/artifact"})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if repo.replaceCalls != 0 {
				t.Fatalf("ReplaceServingStateBindings() calls = %d, want 0", repo.replaceCalls)
			}
		})
	}
}

func TestBinderSanitizesLoaderAndRepositoryErrors(t *testing.T) {
	secret := "s3://private-bucket/object?token=secret"
	tests := []struct {
		name  string
		build func() *Binder
		want  error
	}{
		{name: "loader", build: func() *Binder {
			return newBinder(&fakeRepository{}, func(string) (servingstatefs.CompiledWorkspaceArtifact, error) {
				return servingstatefs.CompiledWorkspaceArtifact{}, errors.New(secret)
			})
		}, want: ErrArtifactMetadata},
		{name: "collection", build: func() *Binder {
			repo := validFakeRepository()
			repo.collectionErr = errors.New(secret)
			return binderForRepository(repo)
		}, want: ErrRepository},
		{name: "revision", build: func() *Binder {
			repo := validFakeRepository()
			repo.listErr = errors.New(secret)
			return binderForRepository(repo)
		}, want: ErrRepository},
		{name: "replacement", build: func() *Binder {
			repo := validFakeRepository()
			repo.replaceErr = errors.New(secret)
			return binderForRepository(repo)
		}, want: ErrRepository},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.build().AfterArtifactValidation(t.Context(), servingstate.State{ID: "state-1", Environment: "prod"}, servingstate.Validation{RootDir: secret})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if strings.Contains(err.Error(), "private-bucket") || strings.Contains(err.Error(), "token=secret") {
				t.Fatalf("error exposed sensitive metadata: %v", err)
			}
		})
	}
}

func validFakeRepository() *fakeRepository {
	return &fakeRepository{
		collections: map[string]manageddata.Collection{"project-a\x00orders": activeCollection("orders", "orders")},
		revisions:   map[string][]manageddata.Revision{"orders": {{ID: "revision-1", CollectionID: "orders", Digest: digestA, Status: manageddata.RevisionStatusReady}}},
	}
}

func activeCollection(id, connection string) manageddata.Collection {
	return manageddata.Collection{ID: id, ProjectID: "project-a", ConnectionName: connection, Status: manageddata.CollectionStatusActive}
}

func binderForRepository(repo Repository) *Binder {
	return newBinder(repo, func(string) (servingstatefs.CompiledWorkspaceArtifact, error) {
		return compiledArtifact("project-a", map[string]map[string]string{"sales": {"orders": "managed"}}, map[string]string{"orders": digestA}), nil
	})
}

func compiledArtifact(projectID string, models map[string]map[string]string, pins map[string]string) servingstatefs.CompiledWorkspaceArtifact {
	definition := &workspace.Definition{Models: make(map[string]*semanticmodel.Model, len(models))}
	for modelName, connections := range models {
		model := &semanticmodel.Model{Connections: make(map[string]semanticmodel.Connection, len(connections))}
		for connectionName, kind := range connections {
			model.Connections[connectionName] = semanticmodel.Connection{Kind: kind}
		}
		definition.Models[modelName] = model
	}
	artifact := servingstatefs.CompiledWorkspaceArtifact{Version: 1, ProjectID: projectID, Definition: definition, ManagedDataRevisions: pins}
	graphJSON, _ := json.Marshal(artifact.Graph)
	graphDigest := sha256.Sum256(graphJSON)
	artifact.Validation = servingstatefs.CompiledArtifactValidation{Status: "passed", SchemaVersion: "libredash.dev/v1", GraphHash: hex.EncodeToString(graphDigest[:])}
	return artifact
}

type fakeRepository struct {
	collections       map[string]manageddata.Collection
	revisions         map[string][]manageddata.Revision
	collectionErr     error
	listErr           error
	replaceErr        error
	collectionLookups []string
	replaced          []manageddata.ServingStateBinding
	listCalls         int
	replaceCalls      int
}

func (r *fakeRepository) CollectionByProjectConnection(_ context.Context, projectID, connectionName string) (manageddata.Collection, error) {
	key := projectID + "\x00" + connectionName
	r.collectionLookups = append(r.collectionLookups, key)
	if r.collectionErr != nil {
		return manageddata.Collection{}, r.collectionErr
	}
	collection, ok := r.collections[key]
	if !ok {
		return manageddata.Collection{}, manageddata.ErrNotFound
	}
	return collection, nil
}

func (r *fakeRepository) ListRevisions(_ context.Context, collectionID string) ([]manageddata.Revision, error) {
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]manageddata.Revision(nil), r.revisions[collectionID]...), nil
}

func (r *fakeRepository) ReplaceServingStateBindings(_ context.Context, _ string, bindings []manageddata.ServingStateBinding) error {
	r.replaceCalls++
	r.replaced = append([]manageddata.ServingStateBinding(nil), bindings...)
	return r.replaceErr
}
