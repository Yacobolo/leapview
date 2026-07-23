package module

import (
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/workspace"
)

func TestPersistenceOwnsWorkspaceSQLiteAdapter(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	persistence, err := BuildPersistence(store.SQLDB(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.Ensure(t.Context(), workspace.EnsureInput{ID: "sales", Title: "Sales"}); err != nil {
		t.Fatal(err)
	}
	ids, err := persistence.WorkspaceIDs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "sales" {
		t.Fatalf("workspace IDs = %v", ids)
	}
}
