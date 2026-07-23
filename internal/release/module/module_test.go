package module

import (
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
)

func TestReleaseStoresAreConstructedInsideModule(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	releases, catalog, deployments, err := releaseStores(Config{Database: store.SQLDB()})
	if err != nil {
		t.Fatal(err)
	}
	if releases == nil || catalog == nil || deployments == nil {
		t.Fatalf("release stores = %#v, %#v, %#v", releases, catalog, deployments)
	}
}

func TestReleaseStoresRequireDatabaseOrExplicitPorts(t *testing.T) {
	if _, _, _, err := releaseStores(Config{}); err == nil {
		t.Fatal("release module accepted missing persistence")
	}
}
