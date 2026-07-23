package module

import (
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
)

func TestBuildRejectsIncompleteOwnedDeploymentComposition(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := Build(t.Context(), Config{Database: store.SQLDB()}); err == nil {
		t.Fatal("deployment module accepted a database without its required capability ports")
	}
}
