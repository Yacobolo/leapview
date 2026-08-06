package sharedassets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheRootUsesOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "shared")
	got, err := CacheRoot(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CacheRoot() = %q, want %q", got, want)
	}
}

func TestEnsureMigratesExistingAssetsAndLinksWorktree(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "worktree", ".data", "assets")
	shared := filepath.Join(root, "cache", "assets", "v1")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "ready"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	populated := false
	err := Ensure(Options{
		Local:  local,
		Shared: shared,
		Ready:  readyFile,
		Populate: func(string) error {
			populated = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if populated {
		t.Fatal("Populate was called despite a valid existing local directory")
	}
	assertSymlinkTarget(t, local, shared)
	data, err := os.ReadFile(filepath.Join(shared, "ready"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("migrated data = %q, want existing", data)
	}
}

func TestEnsureReusesSharedAssetsAcrossWorktrees(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "cache", "assets", "v1")
	populateCalls := 0
	populate := func(target string) error {
		populateCalls++
		return os.WriteFile(filepath.Join(target, "ready"), []byte("shared"), 0o644)
	}

	for _, name := range []string{"one", "two"} {
		local := filepath.Join(root, name, ".data", "assets")
		if err := Ensure(Options{Local: local, Shared: shared, Ready: readyFile, Populate: populate}); err != nil {
			t.Fatal(err)
		}
		assertSymlinkTarget(t, local, shared)
	}
	if populateCalls != 1 {
		t.Fatalf("Populate calls = %d, want 1", populateCalls)
	}
}

func TestEnsureReplacesIncompleteLocalDirectoryAfterSharedPopulation(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "worktree", ".data", "assets")
	shared := filepath.Join(root, "cache", "assets", "v1")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "partial"), []byte("discard me"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Ensure(Options{
		Local:  local,
		Shared: shared,
		Ready:  readyFile,
		Populate: func(target string) error {
			return os.WriteFile(filepath.Join(target, "ready"), []byte("complete"), 0o644)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSymlinkTarget(t, local, shared)
	if _, err := os.Stat(filepath.Join(local, "partial")); !os.IsNotExist(err) {
		t.Fatalf("partial file should be removed, stat error = %v", err)
	}
}

func readyFile(root string) error {
	_, err := os.Stat(filepath.Join(root, "ready"))
	return err
}

func assertSymlinkTarget(t *testing.T, local, want string) {
	t.Helper()
	info, err := os.Lstat(local)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", local)
	}
	got, err := os.Readlink(local)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("symlink target = %q, want %q", got, want)
	}
}
