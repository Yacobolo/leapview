package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/flidai/leapview/rtest/internal/workspace"
)

func TestFilesSelectsExactNonIgnoredWorktree(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	write(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	write(t, filepath.Join(root, "tracked.txt"), "before\n")
	write(t, filepath.Join(root, "deleted.txt"), "delete me\n")
	runGit(t, root, "add", ".gitignore", "tracked.txt", "deleted.txt")
	runGit(t, root, "-c", "user.name=rtest", "-c", "user.email=rtest@example.invalid", "commit", "-qm", "fixture")

	write(t, filepath.Join(root, "tracked.txt"), "dirty bytes\n")
	write(t, filepath.Join(root, "untracked.txt"), "untracked bytes\n")
	write(t, filepath.Join(root, "ignored", "large.bin"), "must stay local\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}

	got, err := workspace.Files(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "tracked.txt", "untracked.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Files() = %#v, want %#v", got, want)
	}
}

func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
