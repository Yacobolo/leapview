package devloop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/internal/platform/digest"
	projectcompiler "github.com/flidai/leapview/internal/project/compiler"
	"github.com/stretchr/testify/require"
)

func BenchmarkFilesystemBuilderCoherentSnapshot(b *testing.B) {
	projectPath, editable := copyBenchmarkProject(b)
	for _, edit := range []struct {
		name  string
		files int
	}{
		{name: "no_edit", files: 0},
		{name: "single_resource_edit", files: 1},
		{name: "multi_resource_edit", files: min(5, len(editable))},
	} {
		b.Run(edit.name, func(b *testing.B) {
			builder := FilesystemBuilder{ProjectPath: projectPath}
			b.ReportAllocs()
			b.ReportMetric(float64(len(editable)+1), "resources")
			for iteration := 0; iteration < b.N; iteration++ {
				b.StopTimer()
				for index := 0; index < edit.files; index++ {
					appendDevloopBenchmarkRevision(b, editable[index], iteration)
				}
				b.StartTimer()
				if _, err := builder.Build(context.Background()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func copyBenchmarkProject(b *testing.B) (string, []string) {
	b.Helper()
	original, err := filepath.Abs(filepath.Join("..", "..", "..", "dashboards", "leapview.yaml"))
	if err != nil {
		b.Fatal(err)
	}
	paths, err := projectcompiler.SourceFiles(original)
	if err != nil {
		b.Fatal(err)
	}
	originalRoot := filepath.Dir(original)
	targetRoot := b.TempDir()
	editable := make([]string, 0, len(paths)-1)
	for _, source := range paths {
		relative, err := filepath.Rel(originalRoot, source)
		if err != nil {
			b.Fatal(err)
		}
		target := filepath.Join(targetRoot, relative)
		body, err := os.ReadFile(source)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			b.Fatal(err)
		}
		if source != original {
			editable = append(editable, target)
		}
	}
	return filepath.Join(targetRoot, "leapview.yaml"), editable
}

func appendDevloopBenchmarkRevision(b *testing.B, path string, revision int) {
	b.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, []byte(fmt.Sprintf("\n# benchmark revision %d\n", revision))...), 0o600); err != nil {
		b.Fatal(err)
	}
}

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
