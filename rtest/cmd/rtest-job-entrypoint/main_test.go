package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitializeGitBaselineMakesMaterializedSnapshotClean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	workspace := t.TempDir()
	tracked := filepath.Join(workspace, "tracked.txt")
	ignored := filepath.Join(workspace, "generated", "contract.json")
	if err := os.WriteFile(tracked, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".gitignore"), []byte("generated/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(ignored), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("snapshot\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := initializeGitBaseline(context.Background(), workspace, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := gitStatus(t, workspace); got != "" {
		t.Fatalf("initial status = %q, want clean snapshot", got)
	}
	if err := os.WriteFile(ignored, []byte("regenerated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := gitStatus(t, workspace); !strings.Contains(got, "generated/contract.json") {
		t.Fatalf("status after changing ignored snapshot file = %q, want tracked change", got)
	}
}

func TestInitializeGitBaselineAcceptsEmptySnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	workspace := t.TempDir()
	if err := initializeGitBaseline(context.Background(), workspace, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := gitStatus(t, workspace); got != "" {
		t.Fatalf("initial status = %q, want clean empty snapshot", got)
	}
}

func gitStatus(t *testing.T, workspace string) string {
	t.Helper()
	command := exec.Command("git", "status", "--porcelain")
	command.Dir = workspace
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v: %s", err, output)
	}
	return string(output)
}

func TestPrepareJobDirectoryKeepsMetadataPrivate(t *testing.T) {
	jobDirectory := filepath.Join(t.TempDir(), "job")
	if err := prepareJobDirectory(jobDirectory, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(jobDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("%s mode = %v, want %v", jobDirectory, got, os.FileMode(0o700))
	}
}

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

func TestHostIdentityFromEnvironment(t *testing.T) {
	t.Setenv("RTEST_HOST_UID", "123")
	t.Setenv("RTEST_HOST_GID", "456")
	uid, gid, err := hostIdentityFromEnvironment()
	if err != nil || uid != 123 || gid != 456 {
		t.Fatalf("uid=%d gid=%d err=%v", uid, gid, err)
	}
	t.Setenv("RTEST_HOST_UID", "invalid")
	if _, _, err := hostIdentityFromEnvironment(); err == nil {
		t.Fatal("invalid host identity was accepted")
	}
}
