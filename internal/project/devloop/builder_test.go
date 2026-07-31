package devloop

import (
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/internal/platform/digest"
	"github.com/stretchr/testify/require"
)

func TestFilesystemBuilderProducesDeterministicWorkspaceArtifacts(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	builder := FilesystemBuilder{ProjectPath: projectPath}

	first, err := builder.Build(t.Context())
	require.NoError(t, err)
	second, err := builder.Build(t.Context())
	require.NoError(t, err)
	if first.ProjectID != "leapview-showcase" ||
		first.ProjectFile != "leapview.yaml" ||
		first.Digest != second.Digest {
		t.Fatalf(
			"candidate identities = (%q, %q) and (%q, %q)",
			first.ProjectID, first.Digest, second.ProjectID, second.Digest,
		)
	}
	if len(first.Artifacts) < 2 {
		t.Fatalf("content artifacts = %d, want reachable project sources", len(first.Artifacts))
	}
	for index, artifact := range first.Artifacts {
		if err := digest.ValidateSHA256Identity(artifact.Digest); err != nil {
			t.Fatalf("artifact %q digest: %v", artifact.Path, err)
		}
		if len(artifact.Content) == 0 ||
			artifact.Path != second.Artifacts[index].Path ||
			artifact.Digest != second.Artifacts[index].Digest {
			t.Fatalf("non-deterministic artifact at %d: %#v / %#v", index, artifact, second.Artifacts[index])
		}
	}
}

func TestCandidateSetDigestIncludesProjectEntrypoint(t *testing.T) {
	artifacts := []Artifact{
		contentArtifact("leapview.yaml", []byte("one")),
		contentArtifact("alternate.yaml", []byte("two")),
	}
	first := candidateSetDigest("project", "leapview.yaml", artifacts)
	second := candidateSetDigest("project", "alternate.yaml", artifacts)
	if first == second {
		t.Fatalf("candidate set digest ignored project entrypoint: %q", first)
	}
}

func TestNormalizeSnapshotRejectsContentAndCandidateSetDigestMismatch(t *testing.T) {
	valid := testSnapshot("valid")

	badContent := cloneSnapshot(valid)
	badContent.Artifacts[0].Content = []byte("tampered")
	if _, err := normalizeSnapshot(badContent); err == nil {
		t.Fatal("normalize snapshot accepted content that does not match its digest")
	}

	badSet := cloneSnapshot(valid)
	badSet.Digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := normalizeSnapshot(badSet); err == nil {
		t.Fatal("normalize snapshot accepted a candidate-set digest that does not match its artifacts")
	}
}

func TestNormalizeSnapshotRejectsUnsafeArtifactPaths(t *testing.T) {
	for _, path := range []string{"../secrets.env", "/etc/passwd", `C:\secrets.env`, "models/../leapview.yaml"} {
		snapshot := testSnapshot("valid")
		snapshot.Artifacts[0].Path = path
		snapshot.Digest = candidateSetDigest(snapshot.ProjectID, snapshot.ProjectFile, snapshot.Artifacts)
		if _, err := normalizeSnapshot(snapshot); err == nil {
			t.Errorf("normalize snapshot accepted unsafe artifact path %q", path)
		}
	}
}

func TestCandidateSetDigestIsIndependentOfWorkspaceOrder(t *testing.T) {
	artifacts := []Artifact{
		{Path: "sales.yaml", Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{Path: "operations.yaml", Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	first := candidateSetDigest("project", "leapview.yaml", artifacts)
	artifacts[0], artifacts[1] = artifacts[1], artifacts[0]
	if second := candidateSetDigest("project", "leapview.yaml", artifacts); second != first {
		t.Fatalf("candidate set digests differ: %q / %q", first, second)
	}
}
