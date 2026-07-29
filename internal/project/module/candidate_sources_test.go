package module_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/internal/project"
	projectdevloop "github.com/flidai/leapview/internal/project/devloop"
	projectmodule "github.com/flidai/leapview/internal/project/module"
)

func TestCandidateSourceSynchronizerAuthorizesOnlyPlannedOwnerUploads(t *testing.T) {
	snapshot, err := (projectdevloop.FilesystemBuilder{
		ProjectPath: filepath.Join("..", "..", "..", "dashboards", "leapview.yaml"),
	}).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	synchronizer, err := projectmodule.NewCandidateSourceSynchronizer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := project.CandidateSourceScope{ProjectID: snapshot.ProjectID, OwnerID: "principal_1"}
	request := synchronizationRequest(snapshot)

	missing, err := synchronizer.Plan(t.Context(), scope, request)
	if err != nil || len(missing) != len(snapshot.Artifacts) {
		t.Fatalf("plan missing=%d error=%v", len(missing), err)
	}
	if err := synchronizer.Upload(t.Context(), project.CandidateSourceScope{
		ProjectID: snapshot.ProjectID, OwnerID: "principal_2",
	}, missing[0], bytes.NewReader(snapshot.Artifacts[0].Content)); err == nil {
		t.Fatal("foreign principal uploaded against another author's plan")
	}
	byDigest := make(map[string]projectdevloop.Artifact, len(snapshot.Artifacts))
	for _, artifact := range snapshot.Artifacts {
		byDigest[artifact.Digest] = artifact
	}
	for _, identity := range missing {
		artifact := byDigest[identity]
		if err := synchronizer.Upload(t.Context(), scope, identity, bytes.NewReader(artifact.Content)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := synchronizer.Commit(t.Context(), scope, request); err != nil {
		t.Fatal(err)
	}
}

func synchronizationRequest(snapshot projectdevloop.Snapshot) project.CandidateSynchronizationRequest {
	request := project.CandidateSynchronizationRequest{
		ProjectFile: snapshot.ProjectFile, ArtifactDigest: snapshot.Digest,
		Artifacts: make([]project.CandidateSourceArtifact, len(snapshot.Artifacts)),
	}
	for index, artifact := range snapshot.Artifacts {
		request.Artifacts[index] = project.CandidateSourceArtifact{
			Path: artifact.Path, Digest: artifact.Digest,
		}
	}
	return request
}
