package platform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstanceIDIsGeneratedOnceAndPersists(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leapview.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.InstanceID(ctx)
	if err != nil {
		t.Fatalf("first InstanceID() error = %v", err)
	}
	if !strings.HasPrefix(first, "lvinst_") || len(first) < 32 {
		t.Fatalf("instance ID = %q", first)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	second, err := reopened.InstanceID(ctx)
	if err != nil {
		t.Fatalf("reopened InstanceID() error = %v", err)
	}
	if second != first {
		t.Fatalf("reopened instance ID = %q, want %q", second, first)
	}
}
