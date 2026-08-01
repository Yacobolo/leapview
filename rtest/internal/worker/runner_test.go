package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/klauspost/compress/zstd"
)

func TestExtractRejectsTraversal(t *testing.T) {
	var compressed bytes.Buffer
	encoder, _ := zstd.NewWriter(&compressed)
	archive := tar.NewWriter(encoder)
	if err := archive.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	_, _ = archive.Write([]byte("x"))
	_ = archive.Close()
	_ = encoder.Close()

	err := extract(bytes.NewReader(compressed.Bytes()), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("extract error = %v", err)
	}
}

func TestRunnerRejectsCorruptSourcePayload(t *testing.T) {
	runner := Runner{Docker: "does-not-matter", WorkRoot: t.TempDir(), Image: "runner@sha256:abc"}
	result := runner.Run(context.Background(), protocol.Job{
		ID: "corrupt", SourceDigest: "sha256:" + strings.Repeat("0", 64),
		Command: []string{"true"}, TimeoutSeconds: 30,
	}, validArchive(t, map[string]string{"go.mod": "module example.test/proof\n"}), &bytes.Buffer{})
	if result.Status != protocol.StatusLost || !strings.Contains(result.ErrorMessage, "checksum") {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunnerUsesIsolatedDockerContract(t *testing.T) {
	root := t.TempDir()
	arguments := filepath.Join(root, "arguments")
	cleanup := filepath.Join(root, "cleanup")
	fakeDocker := filepath.Join(root, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = run ]; then printf '%s\\n' \"$@\" > " + arguments + "; else printf '%s\\n' \"$@\" >> " + cleanup + "; fi\nexit 0\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	source := validArchive(t, map[string]string{"go.mod": "module example.test/proof\n"})
	runner := Runner{Docker: fakeDocker, WorkRoot: filepath.Join(root, "work"), Image: "runner@sha256:abc", CPUs: "7", Memory: "9g"}
	exit := runner.Run(context.Background(), protocol.Job{ID: "job-1", Command: []string{"go", "test", "./..."}, TimeoutSeconds: 30}, source, &bytes.Buffer{})
	if exit.Status != protocol.StatusSucceeded || exit.ExitCode == nil || *exit.ExitCode != 0 {
		t.Fatalf("finish = %#v", exit)
	}
	got, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	jobRoot := filepath.Join(root, "work", "job-1")
	for _, required := range []string{
		"run\n", "--rm\n", "--network\nhost\n", "/var/run/docker.sock:/var/run/docker.sock\n",
		"rtest.job=job-1\n", jobRoot + "/workspace:" + jobRoot + "/workspace\n",
		jobRoot + "/tmp:" + jobRoot + "/tmp\n", jobRoot + "/data:" + jobRoot + "/data\n",
		"TMPDIR=" + jobRoot + "/tmp\n", "TEST_DATA_DIR=" + jobRoot + "/data\n",
		"runner@sha256:abc\n", "go\ntest\n./...\n",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("docker arguments missing %q:\n%s", required, text)
		}
	}
}

func TestRunnerForceRemovesContainerWhenCancelled(t *testing.T) {
	root := t.TempDir()
	arguments := filepath.Join(root, "arguments")
	fakeDocker := filepath.Join(root, "docker")
	script := `#!/bin/sh
printf '%s\n' "$@" >> "` + arguments + `"
if [ "$1" = run ]; then
  exec sleep 30
fi
`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := Runner{Docker: fakeDocker, WorkRoot: filepath.Join(root, "work"), Image: "runner@sha256:abc"}
	result := runner.Run(ctx, protocol.Job{ID: "cancel-me", Command: []string{"sleep", "30"}, TimeoutSeconds: 30},
		validArchive(t, map[string]string{"go.mod": "module example.test/proof\n"}), &bytes.Buffer{})
	if result.Status != protocol.StatusCancelled {
		t.Fatalf("result = %#v", result)
	}
	got, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "rm\n-f\nrtest-cancel-me\n") {
		t.Fatalf("cleanup arguments missing:\n%s", got)
	}
}

func TestRunnerTimesOutAndRemovesContainer(t *testing.T) {
	root := t.TempDir()
	arguments := filepath.Join(root, "arguments")
	fakeDocker := filepath.Join(root, "docker")
	script := `#!/bin/sh
printf '%s\n' "$@" >> "` + arguments + `"
if [ "$1" = run ]; then
  exec sleep 30
fi
`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := Runner{Docker: fakeDocker, WorkRoot: filepath.Join(root, "work"), Image: "runner@sha256:abc"}
	result := runner.Run(context.Background(), protocol.Job{ID: "time-out", Command: []string{"sleep", "30"}, TimeoutSeconds: 1},
		validArchive(t, map[string]string{"go.mod": "module example.test/proof\n"}), &bytes.Buffer{})
	if result.Status != protocol.StatusTimedOut {
		t.Fatalf("result = %#v", result)
	}
	got, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "rm\n-f\nrtest-time-out\n") {
		t.Fatalf("cleanup arguments missing:\n%s", got)
	}
}

func validArchive(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()
	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(encoder)
	for name, data := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		_, _ = archive.Write([]byte(data))
	}
	_ = archive.Close()
	_ = encoder.Close()
	return bytes.NewReader(compressed.Bytes())
}
