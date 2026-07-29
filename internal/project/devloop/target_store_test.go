package devloop

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTargetStorePlansAndRetainsContentAddressedBlobs(t *testing.T) {
	store, err := NewTargetStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshotWithArtifacts("store", []Artifact{
		contentArtifact("leapview.yaml", []byte("project")),
		contentArtifact("models/orders.yaml", []byte("orders")),
	})
	request := planRequestForSnapshot(snapshot)

	missing, err := store.Missing(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 2 {
		t.Fatalf("missing blobs = %#v, want 2", missing)
	}
	for _, artifact := range snapshot.Artifacts {
		if err := store.Put(t.Context(), artifact.Digest, bytes.NewReader(artifact.Content)); err != nil {
			t.Fatal(err)
		}
	}
	missing, err = store.Missing(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing blobs after upload = %#v", missing)
	}
}

func TestTargetStoreRejectsDigestMismatchWithoutRetainingBlob(t *testing.T) {
	store, err := NewTargetStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact := contentArtifact("leapview.yaml", []byte("expected"))
	if err := store.Put(t.Context(), artifact.Digest, bytes.NewReader([]byte("tampered"))); err == nil {
		t.Fatal("target store accepted bytes that do not match content digest")
	}
	request := planRequestForSnapshot(testSnapshotWithArtifacts("store", []Artifact{artifact}))
	missing, err := store.Missing(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != artifact.Digest {
		t.Fatalf("missing blobs = %#v, want rejected digest", missing)
	}
}

func TestTargetStoreCommitsValidatedSnapshotIdempotently(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	snapshot, err := (FilesystemBuilder{ProjectPath: projectPath}).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	store, err := NewTargetStore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range snapshot.Artifacts {
		if err := store.Put(t.Context(), artifact.Digest, bytes.NewReader(artifact.Content)); err != nil {
			t.Fatal(err)
		}
	}
	request := planRequestForSnapshot(snapshot)

	const workers = 6
	var wait sync.WaitGroup
	results := make(chan StoredSnapshot, workers)
	errors := make(chan error, workers)
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := store.Commit(t.Context(), request)
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var committedPath string
	for result := range results {
		if result.ProjectID != snapshot.ProjectID || result.Digest != snapshot.Digest {
			t.Fatalf("stored snapshot = %#v", result)
		}
		if committedPath == "" {
			committedPath = result.ProjectPath
		} else if result.ProjectPath != committedPath {
			t.Fatalf("idempotent commit paths differ: %q / %q", committedPath, result.ProjectPath)
		}
	}
	if relative, err := filepath.Rel(root, committedPath); err != nil ||
		relative == ".." || filepath.IsAbs(relative) {
		t.Fatalf("committed project path %q escapes store %q", committedPath, root)
	}
	info, err := os.Stat(committedPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("committed project mode = %o, want no group/world access", info.Mode().Perm())
	}
}

func TestTargetStoreCannotCommitWithMissingBlobs(t *testing.T) {
	store, err := NewTargetStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot("missing")
	if _, err := store.Commit(t.Context(), planRequestForSnapshot(snapshot)); err == nil {
		t.Fatal("target store committed a snapshot with missing source blobs")
	}
}

func planRequestForSnapshot(snapshot Snapshot) SynchronizationPlanRequest {
	request := SynchronizationPlanRequest{
		ProjectID: snapshot.ProjectID, ProjectFile: snapshot.ProjectFile,
		ArtifactDigest: snapshot.Digest, Artifacts: make([]ArtifactReference, len(snapshot.Artifacts)),
	}
	for index, artifact := range snapshot.Artifacts {
		request.Artifacts[index] = ArtifactReference{Path: artifact.Path, Digest: artifact.Digest}
	}
	return request
}
