package activate

import (
	"context"
	"errors"
	"testing"

	"github.com/Yacobolo/libredash/internal/deployment"
)

func TestServiceActivatesPreparedRuntime(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{
		deployment: deployment.Deployment{ID: "dep_1", WorkspaceID: "test", Status: deployment.StatusValidated},
	}
	runtime := &fakeRuntime{}
	service := NewService(repo, runtime)

	activated, err := service.Activate(ctx, "dep_1")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if activated.Status != deployment.StatusActive {
		t.Fatalf("status = %q, want active", activated.Status)
	}
	if runtime.prepareID != "dep_1" || runtime.commitCalls != 1 {
		t.Fatalf("runtime prepare=%q commits=%d, want dep_1/1", runtime.prepareID, runtime.commitCalls)
	}
}

func TestServicePrepareFailureLeavesDeploymentUnchanged(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{
		deployment: deployment.Deployment{ID: "dep_1", WorkspaceID: "test", Status: deployment.StatusValidated},
	}
	runtime := &fakeRuntime{prepareErr: errors.New("load failed")}
	service := NewService(repo, runtime)

	if _, err := service.Activate(ctx, "dep_1"); err == nil {
		t.Fatal("expected prepare error")
	}
	if repo.activateCalls != 0 {
		t.Fatalf("activate calls = %d, want 0", repo.activateCalls)
	}
}

func TestServiceRejectsInvalidStatusBeforePrepare(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{
		deployment: deployment.Deployment{ID: "dep_1", WorkspaceID: "test", Status: deployment.StatusPending},
	}
	runtime := &fakeRuntime{}
	service := NewService(repo, runtime)

	if _, err := service.Activate(ctx, "dep_1"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("error = %v, want ErrInvalidStatus", err)
	}
	if runtime.prepareID != "" {
		t.Fatalf("prepared invalid deployment %q", runtime.prepareID)
	}
}

type fakeRepo struct {
	deployment    deployment.Deployment
	activateErr   error
	activateCalls int
}

func (r *fakeRepo) ByID(context.Context, deployment.ID) (deployment.Deployment, error) {
	return r.deployment, nil
}

func (r *fakeRepo) Activate(context.Context, deployment.WorkspaceID, deployment.ID) (deployment.Deployment, error) {
	r.activateCalls++
	if r.activateErr != nil {
		return deployment.Deployment{}, r.activateErr
	}
	r.deployment.Status = deployment.StatusActive
	return r.deployment, nil
}

type fakeRuntime struct {
	prepareID   string
	prepareErr  error
	commitCalls int
}

func (r *fakeRuntime) PrepareDeployment(_ context.Context, deploymentID string) (deployment.PreparedRuntime, error) {
	r.prepareID = deploymentID
	if r.prepareErr != nil {
		return nil, r.prepareErr
	}
	return fakePrepared{}, nil
}

func (r *fakeRuntime) CommitPrepared(deployment.PreparedRuntime) error {
	r.commitCalls++
	return nil
}

type fakePrepared struct{}

func (fakePrepared) Close() error { return nil }
