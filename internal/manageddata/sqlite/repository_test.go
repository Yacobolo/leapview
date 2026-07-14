package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestCollectionIdentityUsesProjectAndConnection(t *testing.T) {
	ctx, _, repo := testRepository(t)
	first, err := repo.CreateCollection(ctx, manageddata.CreateCollectionInput{ID: "orders-a", ProjectID: "project-a", ConnectionName: "warehouse", Name: "Orders A"})
	if err != nil {
		t.Fatalf("create first collection: %v", err)
	}
	second, err := repo.CreateCollection(ctx, manageddata.CreateCollectionInput{ID: "orders-b", ProjectID: "project-b", ConnectionName: "warehouse", Name: "Orders B"})
	if err != nil {
		t.Fatalf("same connection name in another project: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("collections share ID %q", first.ID)
	}

	got, err := repo.CollectionByProjectConnection(ctx, "project-a", "warehouse")
	if err != nil {
		t.Fatalf("lookup collection: %v", err)
	}
	if got.ID != first.ID || got.ProjectID != "project-a" || got.ConnectionName != "warehouse" {
		t.Fatalf("lookup = %#v", got)
	}

	retry, err := repo.CreateCollection(ctx, manageddata.CreateCollectionInput{ID: first.ID, ProjectID: "project-a", ConnectionName: "warehouse", Name: "Orders A"})
	if err != nil {
		t.Fatalf("idempotent create retry: %v", err)
	}
	if retry.ID != first.ID {
		t.Fatalf("retry ID = %q, want %q", retry.ID, first.ID)
	}
	_, err = repo.CreateCollection(ctx, manageddata.CreateCollectionInput{ID: "different", ProjectID: "project-a", ConnectionName: "warehouse", Name: "Conflicting"})
	if !errors.Is(err, manageddata.ErrConflict) {
		t.Fatalf("conflicting project+connection error = %v, want conflict", err)
	}
}

func TestCompleteUploadCreatesImmutableRevisionAtomically(t *testing.T) {
	ctx, store, repo := testRepository(t)
	collection := createCollection(t, ctx, repo, "customers", "project-a", "customers")
	manifest := manageddata.Manifest{Files: []manageddata.File{
		{Path: "customers.csv", Size: 12, SHA256: strings.Repeat("a", 64)},
		{Path: "regions.csv", Size: 7, SHA256: strings.Repeat("b", 64)},
	}}
	session, err := repo.CreateUploadSession(ctx, manageddata.CreateUploadSessionInput{
		ID: "upload-1", CollectionID: collection.ID, Manifest: manifest, StorageBackend: "local",
		StagingPrefix: "sessions/upload-1", CreatedBy: "principal-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create upload session: %v", err)
	}

	revision, err := repo.CompleteUpload(ctx, manageddata.CompleteUploadInput{SessionID: session.ID, Files: []manageddata.StoredFile{
		{File: manifest.Files[1], StorageKey: "objects/b", MediaType: "text/csv", ETag: "etag-b"},
		{File: manifest.Files[0], StorageKey: "objects/a", MediaType: "text/csv", ETag: "etag-a"},
	}})
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if revision.Digest != manifest.RevisionID() || revision.Status != manageddata.RevisionStatusReady || revision.Sequence != 1 {
		t.Fatalf("revision = %#v", revision)
	}
	files, err := repo.ListRevisionFiles(ctx, revision.ID)
	if err != nil {
		t.Fatalf("list revision files: %v", err)
	}
	if len(files) != 2 || files[0].Path != "customers.csv" || files[1].Path != "regions.csv" {
		t.Fatalf("files = %#v", files)
	}
	completed, err := repo.UploadSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("get upload session: %v", err)
	}
	if completed.Status != manageddata.UploadStatusComplete || completed.RevisionID != revision.ID {
		t.Fatalf("completed session = %#v", completed)
	}
	if _, err := store.ExecContext(ctx, `UPDATE managed_data_revisions SET digest = ? WHERE id = ?`, "sha256:"+strings.Repeat("f", 64), revision.ID); err == nil {
		t.Fatal("ready revision metadata was mutable")
	}
}

func TestCompleteUploadRollsBackWhenStoredFilesDoNotMatchManifest(t *testing.T) {
	ctx, store, repo := testRepository(t)
	collection := createCollection(t, ctx, repo, "orders", "project-a", "orders")
	manifest := manageddata.Manifest{Files: []manageddata.File{{Path: "orders.csv", Size: 4, SHA256: strings.Repeat("a", 64)}}}
	session, err := repo.CreateUploadSession(ctx, manageddata.CreateUploadSessionInput{ID: "upload-bad", CollectionID: collection.ID, Manifest: manifest, StorageBackend: "local", StagingPrefix: "staging/upload-bad", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteUpload(ctx, manageddata.CompleteUploadInput{SessionID: session.ID, Files: []manageddata.StoredFile{{File: manageddata.File{Path: "orders.csv", Size: 5, SHA256: strings.Repeat("b", 64)}, StorageKey: "objects/bad"}}})
	if err == nil {
		t.Fatal("CompleteUpload() unexpectedly succeeded")
	}
	var revisionCount int
	if err := store.QueryRowContext(ctx, `SELECT count(*) FROM managed_data_revisions`).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if revisionCount != 0 {
		t.Fatalf("revision count = %d, want 0", revisionCount)
	}
	got, err := repo.UploadSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manageddata.UploadStatusOpen {
		t.Fatalf("session status = %q, want open", got.Status)
	}
}

func TestActivateRolloutAtomicallyUpdatesMultipleWorkspaceTargets(t *testing.T) {
	ctx, store, repo := testRepository(t)
	collection, revision := readyRevision(t, ctx, repo, "sales", "project-a", "sales", "sales.csv", "a")
	for i := 1; i <= 3; i++ {
		workspaceID := "workspace-" + string(rune('0'+i))
		insertWorkspaceState(t, ctx, store, workspaceID, "old-"+workspaceID, "prod", "active")
		insertServingState(t, ctx, store, workspaceID, "candidate-"+workspaceID, "prod", "validated")
		setActiveState(t, ctx, store, workspaceID, "prod", "old-"+workspaceID)
	}
	rollout, err := repo.CreateRollout(ctx, manageddata.CreateRolloutInput{
		ID: "rollout-1", CollectionID: collection.ID, Environment: "prod", RevisionID: revision.ID, CreatedBy: "principal-1",
		Targets: []manageddata.RolloutTargetInput{
			{WorkspaceID: "workspace-1", ServingStateID: "candidate-workspace-1"},
			{WorkspaceID: "workspace-2", ServingStateID: "candidate-workspace-2"},
			{WorkspaceID: "workspace-3", ServingStateID: "candidate-workspace-3"},
		},
	})
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}
	if len(rollout.Targets) != 3 || rollout.Targets[1].PriorServingStateID != "old-workspace-2" {
		t.Fatalf("targets = %#v", rollout.Targets)
	}

	// The middle candidate becomes invalid after rollout planning. No earlier target may commit.
	if _, err := store.ExecContext(ctx, `UPDATE serving_states SET status = 'failed' WHERE id = 'candidate-workspace-2'`); err != nil {
		t.Fatal(err)
	}
	_, err = repo.ActivateRollout(ctx, rollout.ID, manageddata.PointerExpectation{Generation: 0})
	if !errors.Is(err, manageddata.ErrConflict) {
		t.Fatalf("invalid middle target error = %v, want conflict", err)
	}
	assertRolloutNotApplied(t, ctx, store, collection.ID, "prod")

	if _, err := store.ExecContext(ctx, `UPDATE serving_states SET status = 'validated' WHERE id = 'candidate-workspace-2'`); err != nil {
		t.Fatal(err)
	}
	setActiveState(t, ctx, store, "workspace-2", "prod", "candidate-workspace-2")
	_, err = repo.ActivateRollout(ctx, rollout.ID, manageddata.PointerExpectation{Generation: 0})
	if !errors.Is(err, manageddata.ErrConflict) {
		t.Fatalf("stale workspace pointer error = %v, want conflict", err)
	}
	setActiveState(t, ctx, store, "workspace-2", "prod", "old-workspace-2")
	assertRolloutNotApplied(t, ctx, store, collection.ID, "prod")

	active, err := repo.ActivateRollout(ctx, rollout.ID, manageddata.PointerExpectation{Generation: 0})
	if err != nil {
		t.Fatalf("activate rollout: %v", err)
	}
	if active.Status != manageddata.RolloutStatusActive {
		t.Fatalf("rollout status = %q", active.Status)
	}
	for _, target := range active.Targets {
		if target.Status != manageddata.TargetStatusActive {
			t.Fatalf("target %s status = %q", target.WorkspaceID, target.Status)
		}
	}
	pointer, err := repo.EnvironmentPointer(ctx, collection.ID, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if pointer.RevisionID != revision.ID || pointer.Generation != 1 {
		t.Fatalf("environment pointer = %#v", pointer)
	}
	for i := 1; i <= 3; i++ {
		workspaceID := "workspace-" + string(rune('0'+i))
		candidateID := "candidate-" + workspaceID
		assertActiveState(t, ctx, store, workspaceID, "prod", candidateID)
		assertServingStateStatus(t, ctx, store, candidateID, "active")
		assertServingStateStatus(t, ctx, store, "old-"+workspaceID, "draining")
		bindings, err := repo.ListServingStateBindings(ctx, candidateID)
		if err != nil || len(bindings) != 1 || bindings[0].RevisionID != revision.ID {
			t.Fatalf("bindings for %s = %#v, err=%v", candidateID, bindings, err)
		}
	}
}

func TestRolloutCanReactivateInactiveServingState(t *testing.T) {
	ctx, store, repo := testRepository(t)
	collection, revision := readyRevision(t, ctx, repo, "sales", "project-a", "sales", "sales.csv", "a")
	insertWorkspaceState(t, ctx, store, "workspace-1", "current-state", "prod", "active")
	insertServingState(t, ctx, store, "workspace-1", "prior-state", "prod", "inactive")
	setActiveState(t, ctx, store, "workspace-1", "prod", "current-state")

	rollout, err := repo.CreateRollout(ctx, manageddata.CreateRolloutInput{
		ID: "rollout-rollback", CollectionID: collection.ID, Environment: "prod", RevisionID: revision.ID,
		Targets: []manageddata.RolloutTargetInput{{WorkspaceID: "workspace-1", ServingStateID: "prior-state"}},
	})
	if err != nil {
		t.Fatalf("create rollback rollout: %v", err)
	}
	if _, err := repo.ActivateRollout(ctx, rollout.ID, manageddata.PointerExpectation{}); err != nil {
		t.Fatalf("activate rollback rollout: %v", err)
	}
	assertActiveState(t, ctx, store, "workspace-1", "prod", "prior-state")
	assertServingStateStatus(t, ctx, store, "prior-state", "active")
	assertServingStateStatus(t, ctx, store, "current-state", "draining")
}

func TestServingStateBindingsAllowMultipleCollections(t *testing.T) {
	ctx, store, repo := testRepository(t)
	firstCollection, firstRevision := readyRevision(t, ctx, repo, "inventory", "project-a", "inventory", "inventory.csv", "c")
	secondCollection, secondRevision := readyRevision(t, ctx, repo, "prices", "project-a", "prices", "prices.csv", "d")
	insertWorkspaceState(t, ctx, store, "workspace-1", "state-1", "prod", "validated")
	bindings := []manageddata.ServingStateBinding{
		{CollectionID: firstCollection.ID, RevisionID: firstRevision.ID, Environment: "prod"},
		{CollectionID: secondCollection.ID, RevisionID: secondRevision.ID, Environment: "prod"},
	}
	if err := repo.ReplaceServingStateBindings(ctx, "state-1", bindings); err != nil {
		t.Fatalf("replace bindings: %v", err)
	}
	got, err := repo.ListServingStateBindings(ctx, "state-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].CollectionID != firstCollection.ID || got[1].CollectionID != secondCollection.ID {
		t.Fatalf("bindings = %#v", got)
	}
}

func testRepository(t *testing.T) (context.Context, *sql.DB, *Repository) {
	t.Helper()
	ctx := context.Background()
	store, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "libredash.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	store.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, store, "../../platform/migrations"); err != nil {
		_ = store.Close()
		t.Fatalf("migrate platform store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return ctx, store, NewRepository(store)
}

func createCollection(t *testing.T, ctx context.Context, repo *Repository, id, projectID, connectionName string) manageddata.Collection {
	t.Helper()
	collection, err := repo.CreateCollection(ctx, manageddata.CreateCollectionInput{ID: id, ProjectID: projectID, ConnectionName: connectionName, Name: connectionName})
	if err != nil {
		t.Fatal(err)
	}
	return collection
}

func readyRevision(t *testing.T, ctx context.Context, repo *Repository, id, projectID, connectionName, path, digestChar string) (manageddata.Collection, manageddata.Revision) {
	t.Helper()
	collection := createCollection(t, ctx, repo, id, projectID, connectionName)
	manifest := manageddata.Manifest{Files: []manageddata.File{{Path: path, Size: 1, SHA256: strings.Repeat(digestChar, 64)}}}
	session, err := repo.CreateUploadSession(ctx, manageddata.CreateUploadSessionInput{CollectionID: collection.ID, Manifest: manifest, StorageBackend: "local", StagingPrefix: "staging/" + path, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.CompleteUpload(ctx, manageddata.CompleteUploadInput{SessionID: session.ID, Files: []manageddata.StoredFile{{File: manifest.Files[0], StorageKey: "objects/" + digestChar}}})
	if err != nil {
		t.Fatal(err)
	}
	return collection, revision
}

func insertWorkspaceState(t *testing.T, ctx context.Context, db *sql.DB, workspaceID, stateID, environment, status string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO workspaces (id, title) VALUES (?, ?)`, workspaceID, workspaceID); err != nil {
		t.Fatal(err)
	}
	insertServingState(t, ctx, db, workspaceID, stateID, environment, status)
}

func insertServingState(t *testing.T, ctx context.Context, db *sql.DB, workspaceID, stateID, environment, status string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO serving_states (id, workspace_id, environment, status, source) VALUES (?, ?, ?, ?, 'publish')`, stateID, workspaceID, environment, status); err != nil {
		t.Fatal(err)
	}
}

func setActiveState(t *testing.T, ctx context.Context, db *sql.DB, workspaceID, environment, stateID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO workspace_active_serving_states (workspace_id, environment, serving_state_id) VALUES (?, ?, ?) ON CONFLICT(workspace_id, environment) DO UPDATE SET serving_state_id = excluded.serving_state_id`, workspaceID, environment, stateID); err != nil {
		t.Fatal(err)
	}
}

func assertRolloutNotApplied(t *testing.T, ctx context.Context, db *sql.DB, collectionID, environment string) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM managed_data_environment_pointers WHERE collection_id = ? AND environment = ?`, collectionID, environment).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("collection environment pointer changed despite rollback")
	}
	for i := 1; i <= 3; i++ {
		workspaceID := "workspace-" + string(rune('0'+i))
		assertActiveState(t, ctx, db, workspaceID, environment, "old-"+workspaceID)
		var status string
		if err := db.QueryRowContext(ctx, `SELECT status FROM serving_states WHERE id = ?`, "candidate-"+workspaceID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if workspaceID != "workspace-2" && status != "validated" {
			t.Fatalf("candidate %s status = %q", workspaceID, status)
		}
	}
}

func assertActiveState(t *testing.T, ctx context.Context, db *sql.DB, workspaceID, environment, want string) {
	t.Helper()
	var got string
	if err := db.QueryRowContext(ctx, `SELECT serving_state_id FROM workspace_active_serving_states WHERE workspace_id = ? AND environment = ?`, workspaceID, environment).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("active serving state for %s = %q, want %q", workspaceID, got, want)
	}
}

func assertServingStateStatus(t *testing.T, ctx context.Context, db *sql.DB, stateID, want string) {
	t.Helper()
	var got string
	if err := db.QueryRowContext(ctx, `SELECT status FROM serving_states WHERE id = ?`, stateID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("serving state %s status = %q, want %q", stateID, got, want)
	}
}
