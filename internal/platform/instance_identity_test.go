package platform

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstanceIDIsGeneratedOnceAndPersists(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leapview.db")
	store, err := Open(ctx, path)
	require.NoError(t, err)
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
	require.NoError(t, err)
	defer reopened.Close()
	second, err := reopened.InstanceID(ctx)
	if err != nil {
		t.Fatalf("reopened InstanceID() error = %v", err)
	}
	if second != first {
		t.Fatalf("reopened instance ID = %q, want %q", second, first)
	}
}

func TestInstanceIDIsStableUnderConcurrentInitialization(t *testing.T) {
	store, err := Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	require.NoError(t, err)
	defer store.Close()

	const callers = 16
	ids := make(chan string, callers)
	errs := make(chan error, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id, err := store.InstanceID(t.Context())
			ids <- id
			errs <- err
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var want string
	for id := range ids {
		if want == "" {
			want = id
		}
		if id != want {
			t.Fatalf("concurrent instance ID = %q, want %q", id, want)
		}
	}
}
