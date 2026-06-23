package runtimehost

import (
	"context"
	"errors"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/deployment"
)

func TestManagerReloadIgnoresMissingActiveDeployment(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRepo{activeErr: deployment.ErrNotFound}, "test", "/data", &fakeFactory{})

	if err := manager.Reload(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

func TestManagerPrepareCommitSwapsRuntimeAndClosesOld(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{
		deployment: deployment.Deployment{ID: "dep_1", WorkspaceID: "test", Status: deployment.StatusValidated},
		artifact:   deployment.Artifact{DeploymentID: "dep_1", Digest: "digest"},
	}
	factory := &fakeFactory{}
	manager := NewManagerWithFactory(repo, "test", "/data", factory)

	prepared, err := manager.PrepareDeployment(ctx, "dep_1")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := manager.CommitPrepared(prepared); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if manager.DefaultDashboardID() != "dashboard" {
		t.Fatalf("default dashboard = %q, want dashboard", manager.DefaultDashboardID())
	}

	second, err := manager.PrepareDeployment(ctx, "dep_1")
	if err != nil {
		t.Fatalf("prepare second: %v", err)
	}
	if err := manager.CommitPrepared(second); err != nil {
		t.Fatalf("commit second: %v", err)
	}
	if factory.prepareCalls != 1 {
		t.Fatalf("factory calls = %d, want no-change reuse", factory.prepareCalls)
	}
}

func TestManagerRejectsPreparedFromDifferentHost(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRepo{}, "test", "/data", &fakeFactory{})
	if err := manager.CommitPrepared(fakePrepared{}); err == nil {
		t.Fatal("expected wrong prepared runtime error")
	}
}

type fakeRepo struct {
	deployment deployment.Deployment
	artifact   deployment.Artifact
	activeErr  error
}

func (r *fakeRepo) ActiveArtifact(context.Context, deployment.WorkspaceID) (deployment.Deployment, deployment.Artifact, error) {
	if r.activeErr != nil {
		return deployment.Deployment{}, deployment.Artifact{}, r.activeErr
	}
	return r.deployment, r.artifact, nil
}

func (r *fakeRepo) ByID(context.Context, deployment.ID) (deployment.Deployment, error) {
	if r.deployment.ID == "" {
		return deployment.Deployment{}, deployment.ErrNotFound
	}
	return r.deployment, nil
}

func (r *fakeRepo) ArtifactByDeployment(context.Context, deployment.ID) (deployment.Artifact, error) {
	if r.artifact.Digest == "" {
		return deployment.Artifact{}, deployment.ErrNotFound
	}
	return r.artifact, nil
}

type fakeFactory struct {
	prepareCalls int
	err          error
}

func (f *fakeFactory) Prepare(context.Context, RuntimeInput) (Runtime, error) {
	f.prepareCalls++
	if f.err != nil {
		return nil, f.err
	}
	return &fakeRuntime{}, nil
}

type fakeRuntime struct {
	closed bool
}

func (r *fakeRuntime) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRuntime) Catalog() dashboard.Catalog        { return dashboard.Catalog{} }
func (r *fakeRuntime) DefaultDashboardID() string        { return "dashboard" }
func (r *fakeRuntime) ModelIDForDashboard(string) string { return "" }
func (r *fakeRuntime) Report(string) (reportdef.Dashboard, *semanticmodel.Model, bool) {
	return reportdef.Dashboard{}, nil, false
}
func (r *fakeRuntime) DefaultFilters(string) dashboard.Filters { return dashboard.Filters{} }
func (r *fakeRuntime) NormalizeTableRequest(_ string, request dashboard.TableRequest) dashboard.TableRequest {
	return request
}
func (r *fakeRuntime) QueryDashboardPage(context.Context, string, string, dashboard.Filters) (dashboard.Patch, error) {
	return dashboard.Patch{}, nil
}
func (r *fakeRuntime) QueryTablePage(context.Context, string, string, dashboard.Filters, dashboard.TableRequest) (dashboard.Table, error) {
	return dashboard.Table{}, nil
}
func (r *fakeRuntime) RefreshMaterializations(context.Context, string) error { return nil }
func (r *fakeRuntime) Pages(string) []dashboard.Page                         { return nil }

type fakePrepared struct{}

func (fakePrepared) Close() error { return errors.New("unused") }
