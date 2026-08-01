package swarmscheduler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/rtest/internal/control"
)

func TestProjectCachesUseIndependentPrivateDirectories(t *testing.T) {
	root := t.TempDir()
	caches := []control.CacheMount{{Name: "modules", Target: "/go/pkg/mod"}}
	for _, project := range []string{"project-one", "project-two"} {
		if err := prepareCacheDirectories(root, project, caches); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(root, project, "modules"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("cache mode = %o", info.Mode().Perm())
		}
	}
	one := specForJob(Config{CacheRoot: root}, control.Job{ID: "job-one", ProjectID: "project-one", Caches: caches})
	two := specForJob(Config{CacheRoot: root}, control.Job{ID: "job-two", ProjectID: "project-two", Caches: caches})
	if filepath.Join(one.CacheRoot, one.ProjectID, one.Caches[0].Name) == filepath.Join(two.CacheRoot, two.ProjectID, two.Caches[0].Name) {
		t.Fatal("two projects resolved to the same writable cache")
	}
}

func TestCacheDirectoriesRejectUnsafeComponents(t *testing.T) {
	if err := prepareCacheDirectories(t.TempDir(), "project", []control.CacheMount{{Name: "../shared", Target: "/cache"}}); err == nil {
		t.Fatal("unsafe cache name was accepted")
	}
}
