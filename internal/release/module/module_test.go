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

	releases, finalization, catalog, deployments, err := releaseStores(store.SQLDB())
	if err != nil {
		t.Fatal(err)
	}
	if releases == nil || finalization == nil || catalog == nil || deployments == nil {
		t.Fatalf("release stores = %#v, %#v, %#v, %#v", releases, finalization, catalog, deployments)
	}
}

func TestReleaseStoresRequireDatabase(t *testing.T) {
	if _, _, _, _, err := releaseStores(nil); err == nil {
		t.Fatal("release module accepted missing persistence")
	}
}
