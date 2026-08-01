package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerSlotSerializesJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.lock")
	releaseFirst, err := acquireWorkerSlot(context.Background(), path, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	acquired := make(chan func(), 1)
	errs := make(chan error, 1)
	go func() {
		release, err := acquireWorkerSlot(context.Background(), path, io.Discard)
		if err != nil {
			errs <- err
			return
		}
		acquired <- release
	}()

	select {
	case <-acquired:
		t.Fatal("second job acquired the worker slot before the first released it")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseFirst()
	select {
	case release := <-acquired:
		release()
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("second job did not acquire the released worker slot")
	}
}

func TestWorkerSlotWaitHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.lock")
	release, err := acquireWorkerSlot(context.Background(), path, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = acquireWorkerSlot(ctx, path, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
