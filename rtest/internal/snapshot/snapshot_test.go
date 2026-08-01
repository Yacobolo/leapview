package snapshot_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/flidai/leapview/rtest/internal/snapshot"
	"github.com/klauspost/compress/zstd"
)

func TestCreateCapturesExactDirtyWorktree(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.email", "rtest@example.test")
	git(t, repo, "config", "user.name", "Remote Test Runner")

	write(t, repo, "kept.txt", "baseline\n", 0o644)
	write(t, repo, "deleted.txt", "remove me\n", 0o644)
	write(t, repo, ".gitignore", "ignored.txt\ncache/\n", 0o644)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "baseline")

	write(t, repo, "kept.txt", "dirty contents\n", 0o644)
	write(t, repo, "untracked.txt", "untracked contents\n", 0o600)
	write(t, repo, "script.sh", "#!/bin/sh\nexit 0\n", 0o755)
	write(t, repo, "ignored.txt", "must not upload\n", 0o644)
	write(t, repo, "cache/object", "must not upload\n", 0o644)
	if err := os.Remove(filepath.Join(repo, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink("kept.txt", filepath.Join(repo, "link")); err != nil {
			t.Fatal(err)
		}
	}

	var compressed bytes.Buffer
	result, err := snapshot.Create(context.Background(), repo, &compressed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Digest == "" || result.Size != int64(compressed.Len()) {
		t.Fatalf("result = %#v, bytes = %d", result, compressed.Len())
	}

	entries := readArchive(t, compressed.Bytes())
	want := map[string]string{
		".gitignore":    "ignored.txt\ncache/\n",
		"kept.txt":      "dirty contents\n",
		"script.sh":     "#!/bin/sh\nexit 0\n",
		"untracked.txt": "untracked contents\n",
	}
	if runtime.GOOS != "windows" {
		want["link"] = "symlink:kept.txt"
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	for name, expected := range want {
		if entries[name] != expected {
			t.Errorf("entry %q = %q, want %q", name, entries[name], expected)
		}
	}
	for _, forbidden := range []string{"deleted.txt", "ignored.txt", "cache/object", ".git/config"} {
		if _, ok := entries[forbidden]; ok {
			t.Errorf("archive includes forbidden path %q", forbidden)
		}
	}
}

func TestCreateRejectsNonRepository(t *testing.T) {
	var dst bytes.Buffer
	if _, err := snapshot.Create(context.Background(), t.TempDir(), &dst); err == nil {
		t.Fatal("Create succeeded outside a Git worktree")
	}
}

func readArchive(t *testing.T, compressed []byte) map[string]string {
	t.Helper()
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	reader := tar.NewReader(decoder)
	entries := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch header.Typeflag {
		case tar.TypeReg:
			contents, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			entries[header.Name] = string(contents)
		case tar.TypeSymlink:
			entries[header.Name] = "symlink:" + header.Linkname
		}
	}
	return entries
}

func write(t *testing.T, root, name, contents string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
