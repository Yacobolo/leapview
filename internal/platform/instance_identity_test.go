package platform

import (
	"path/filepath"
	"regexp"
	"sync"
	"testing"
)

func TestEnsureInstanceIDPersistsAcrossReopen(t *testing.T) {
	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "leapview.db")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	instanceID, err := store.EnsureInstanceID(ctx)
	if err != nil {
		t.Fatalf("ensure instance id: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	store, err = Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close()
	reopenedID, err := store.EnsureInstanceID(ctx)
	if err != nil {
		t.Fatalf("ensure reopened instance id: %v", err)
	}
	if reopenedID != instanceID {
		t.Fatalf("reopened instance id = %q, want %q", reopenedID, instanceID)
	}
	if !regexp.MustCompile(`^instance_[0-9a-f]{32}$`).MatchString(instanceID) {
		t.Fatalf("instance id = %q, want opaque instance_<32 lowercase hex> value", instanceID)
	}
}

func TestEnsureInstanceIDIsStableUnderConcurrentInitialization(t *testing.T) {
	store, err := Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	const callers = 16
	ids := make(chan string, callers)
	errs := make(chan error, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id, err := store.EnsureInstanceID(t.Context())
			ids <- id
			errs <- err
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("ensure instance id concurrently: %v", err)
		}
	}
	var want string
	for id := range ids {
		if want == "" {
			want = id
		}
		if id != want {
			t.Fatalf("concurrent instance id = %q, want %q", id, want)
		}
	}
}
