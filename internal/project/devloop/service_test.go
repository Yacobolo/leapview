package devloop

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestReconcilePreservesLastValidCandidateWhenNextBuildFails(t *testing.T) {
	builder := &scriptedBuilder{steps: []buildStep{
		{snapshot: testSnapshot("first")},
		{err: errors.New("models/orders.yaml:12: unknown source")},
	}}
	remote := &recordingRemote{}
	service, err := New(builder, remote)
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Reconcile(t.Context())
	if err != nil || first.Status != StatusSynchronized {
		t.Fatalf("first reconcile = %#v, %v", first, err)
	}
	next, err := service.Reconcile(t.Context())
	if err == nil || next.Status != StatusInvalid {
		t.Fatalf("invalid reconcile = %#v, %v", next, err)
	}
	if len(remote.requests) != 1 {
		t.Fatalf("remote requests = %d, want only last valid build", len(remote.requests))
	}
	if next.Candidate.ID != first.Candidate.ID ||
		next.Candidate.ArtifactDigest != first.Candidate.ArtifactDigest {
		t.Fatalf("invalid build replaced last valid candidate: first=%#v next=%#v", first, next)
	}
}

func TestReconcileIsIdempotentAndRetriesFailedSynchronization(t *testing.T) {
	snapshot := testSnapshot("retry")
	builder := &scriptedBuilder{steps: []buildStep{
		{snapshot: snapshot},
		{snapshot: snapshot},
		{snapshot: snapshot},
	}}
	remote := &recordingRemote{errors: []error{errors.New("temporary disconnect"), nil}}
	service, err := New(builder, remote)
	if err != nil {
		t.Fatal(err)
	}

	if result, err := service.Reconcile(t.Context()); err == nil || result.Status != StatusRetryable {
		t.Fatalf("first reconcile = %#v, %v, want retryable", result, err)
	}
	synchronized, err := service.Reconcile(t.Context())
	if err != nil || synchronized.Status != StatusSynchronized {
		t.Fatalf("second reconcile = %#v, %v", synchronized, err)
	}
	unchanged, err := service.Reconcile(t.Context())
	if err != nil || unchanged.Status != StatusUnchanged {
		t.Fatalf("third reconcile = %#v, %v", unchanged, err)
	}
	if len(remote.requests) != 2 {
		t.Fatalf("remote requests = %d, want retry plus success", len(remote.requests))
	}
	if remote.requests[0].ExpectedArtifactDigest != "" ||
		remote.requests[1].ExpectedArtifactDigest != "" {
		t.Fatalf("failed synchronization advanced optimistic state: %#v", remote.requests)
	}
}

func TestConcurrentReconcileSerializesOneDigestSynchronization(t *testing.T) {
	builder := &constantBuilder{snapshot: testSnapshot("concurrent")}
	remote := &recordingRemote{}
	service, err := New(builder, remote)
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	results := make(chan Result, 8)
	errors := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Reconcile(t.Context())
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	synchronized := 0
	for result := range results {
		if result.Status == StatusSynchronized {
			synchronized++
		}
	}
	if synchronized != 1 || len(remote.requests) != 1 {
		t.Fatalf("synchronized results = %d, remote requests = %d; want 1, 1", synchronized, len(remote.requests))
	}
}

type buildStep struct {
	snapshot Snapshot
	err      error
}

type scriptedBuilder struct {
	steps []buildStep
	index int
}

func (builder *scriptedBuilder) Build(context.Context) (Snapshot, error) {
	step := builder.steps[builder.index]
	builder.index++
	return step.snapshot, step.err
}

type constantBuilder struct {
	snapshot Snapshot
}

func (builder *constantBuilder) Build(context.Context) (Snapshot, error) {
	return builder.snapshot, nil
}

type recordingRemote struct {
	mu       sync.Mutex
	requests []SyncRequest
	errors   []error
}

func (remote *recordingRemote) Synchronize(_ context.Context, request SyncRequest) (Candidate, error) {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	remote.requests = append(remote.requests, request)
	if len(remote.errors) > 0 {
		err := remote.errors[0]
		remote.errors = remote.errors[1:]
		if err != nil {
			return Candidate{}, err
		}
	}
	return Candidate{
		ID: "cand_1", ProjectID: request.Snapshot.ProjectID,
		ArtifactDigest: request.Snapshot.Digest,
		PreviewURL:     "https://target.example/candidates/cand_1",
	}, nil
}

func testSnapshot(content string) Snapshot {
	artifacts := []Artifact{contentArtifact("leapview.yaml", []byte(content))}
	return Snapshot{
		ProjectID: "sales_project",
		Digest:    candidateSetDigest("sales_project", artifacts),
		Artifacts: artifacts,
	}
}
