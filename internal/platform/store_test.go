package platform

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Yacobolo/libredash/internal/access"
)

func TestStoreMigratesAndSeedsRoles(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "libredash.db")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	store, err = Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen migrated store: %v", err)
	}
	defer store.Close()

	rows, err := store.SQLDB().QueryContext(ctx, `SELECT name FROM roles ORDER BY name`)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			t.Fatalf("scan role: %v", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("roles rows: %v", err)
	}
	defaultRoles := access.DefaultRoles()
	want := make([]string, 0, len(defaultRoles))
	for _, role := range defaultRoles {
		want = append(want, role.Name)
	}
	sort.Strings(want)
	if len(roles) != len(want) {
		t.Fatalf("roles = %#v, want %#v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("roles = %#v, want %#v", roles, want)
		}
	}
}
