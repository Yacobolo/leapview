package runtimehost

import (
	"context"
	"errors"
	"fmt"
	"sync"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/deployment"
)

type DeploymentRepository interface {
	ActiveArtifact(ctx context.Context, workspaceID deployment.WorkspaceID) (deployment.Deployment, deployment.Artifact, error)
	ByID(ctx context.Context, id deployment.ID) (deployment.Deployment, error)
	ArtifactByDeployment(ctx context.Context, deploymentID deployment.ID) (deployment.Artifact, error)
}

type Runtime interface {
	Close() error
	Catalog() dashboard.Catalog
	DefaultDashboardID() string
	ModelIDForDashboard(dashboardID string) string
	Report(dashboardID string) (reportdef.Dashboard, *semanticmodel.Model, bool)
	DefaultFilters(dashboardID string) dashboard.Filters
	NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest
	QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error)
	QueryTablePage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error)
	RefreshMaterializations(ctx context.Context, modelID string) error
	Pages(dashboardID string) []dashboard.Page
}

type RuntimeFactory interface {
	Prepare(ctx context.Context, input RuntimeInput) (Runtime, error)
}

type RuntimeInput struct {
	Deployment deployment.Deployment
	Artifact   deployment.Artifact
	DataDir    string
	DuckDBDir  string
	RuntimeDir string
}

type Manager struct {
	mu          sync.RWMutex
	repo        DeploymentRepository
	workspaceID deployment.WorkspaceID
	dataDir     string
	factory     RuntimeFactory

	activeDeployment deployment.ID
	activeDigest     string
	current          Runtime
}

type Prepared struct {
	deploymentID deployment.ID
	digest       string
	runtime      Runtime
	noChange     bool
}

func (p *Prepared) Close() error {
	if p == nil || p.runtime == nil {
		return nil
	}
	return p.runtime.Close()
}

func NewManagerWithFactory(repo DeploymentRepository, workspaceID deployment.WorkspaceID, dataDir string, factory RuntimeFactory) *Manager {
	return &Manager{
		repo:        repo,
		workspaceID: workspaceID,
		dataDir:     dataDir,
		factory:     factory,
	}
}

func (m *Manager) Reload(ctx context.Context) error {
	current, artifact, err := m.repo.ActiveArtifact(ctx, m.workspaceID)
	if err != nil {
		if errors.Is(err, deployment.ErrNotFound) {
			return nil
		}
		return err
	}
	prepared, err := m.prepare(ctx, current, artifact)
	if err != nil {
		return err
	}
	return m.CommitPrepared(prepared)
}

func (m *Manager) PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error) {
	current, err := m.repo.ByID(ctx, deployment.ID(deploymentID))
	if err != nil {
		return nil, err
	}
	if current.WorkspaceID != m.workspaceID {
		return nil, fmt.Errorf("deployment %s is not in workspace %s", deploymentID, m.workspaceID)
	}
	artifact, err := m.repo.ArtifactByDeployment(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	return m.prepare(ctx, current, artifact)
}

func (m *Manager) prepare(ctx context.Context, current deployment.Deployment, artifact deployment.Artifact) (*Prepared, error) {
	m.mu.RLock()
	if m.current != nil && m.activeDeployment == current.ID && m.activeDigest == artifact.Digest {
		m.mu.RUnlock()
		return &Prepared{deploymentID: current.ID, digest: artifact.Digest, noChange: true}, nil
	}
	m.mu.RUnlock()

	runtime, err := m.factory.Prepare(ctx, RuntimeInput{
		Deployment: current,
		Artifact:   artifact,
		DataDir:    m.dataDir,
	})
	if err != nil {
		return nil, err
	}
	return &Prepared{deploymentID: current.ID, digest: artifact.Digest, runtime: runtime}, nil
}

func (m *Manager) CommitPrepared(candidate deployment.PreparedRuntime) error {
	prepared, ok := candidate.(*Prepared)
	if !ok {
		return fmt.Errorf("prepared runtime belongs to a different host")
	}
	if prepared == nil {
		return fmt.Errorf("prepared runtime is nil")
	}
	if prepared.noChange {
		return nil
	}

	m.mu.Lock()
	old := m.current
	m.current = prepared.runtime
	m.activeDeployment = prepared.deploymentID
	m.activeDigest = prepared.digest
	prepared.runtime = nil
	m.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return nil
	}
	return m.current.Close()
}

func (m *Manager) runtime() (Runtime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return nil, fmt.Errorf("no active LibreDash deployment")
	}
	return m.current, nil
}

func (m *Manager) Catalog() dashboard.Catalog {
	runtime, err := m.runtime()
	if err != nil {
		return dashboard.Catalog{
			Workspace: dashboard.CatalogWorkspace{ID: string(m.workspaceID), Title: "LibreDash Workspace", Description: "No active deployment."},
		}
	}
	return runtime.Catalog()
}

func (m *Manager) DefaultDashboardID() string {
	runtime, err := m.runtime()
	if err != nil {
		return ""
	}
	return runtime.DefaultDashboardID()
}

func (m *Manager) ModelIDForDashboard(dashboardID string) string {
	runtime, err := m.runtime()
	if err != nil {
		return ""
	}
	return runtime.ModelIDForDashboard(dashboardID)
}

func (m *Manager) Report(dashboardID string) (reportdef.Dashboard, *semanticmodel.Model, bool) {
	runtime, err := m.runtime()
	if err != nil {
		return reportdef.Dashboard{}, nil, false
	}
	return runtime.Report(dashboardID)
}

func (m *Manager) DefaultFilters(dashboardID string) dashboard.Filters {
	runtime, err := m.runtime()
	if err != nil {
		return dashboard.Filters{}.WithDefaults()
	}
	return runtime.DefaultFilters(dashboardID)
}

func (m *Manager) NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest {
	runtime, err := m.runtime()
	if err != nil {
		return request.WithDefaults()
	}
	return runtime.NormalizeTableRequest(dashboardID, request)
}

func (m *Manager) QueryDashboard(ctx context.Context, dashboardID string, filters dashboard.Filters) (dashboard.Patch, error) {
	return m.QueryDashboardPage(ctx, dashboardID, "", filters)
}

func (m *Manager) QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error) {
	runtime, err := m.runtime()
	if err != nil {
		return dashboard.EmptyPatch(filters.WithDefaults(), m.dataDir, err), nil
	}
	return runtime.QueryDashboardPage(ctx, dashboardID, pageID, filters)
}

func (m *Manager) QueryTable(ctx context.Context, dashboardID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	return m.QueryTablePage(ctx, dashboardID, "", filters, request)
}

func (m *Manager) QueryTablePage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	runtime, err := m.runtime()
	if err != nil {
		return dashboard.EmptyTable(request.WithDefaults(), err), nil
	}
	return runtime.QueryTablePage(ctx, dashboardID, pageID, filters, request)
}

func (m *Manager) RefreshMaterializations(ctx context.Context, modelID string) error {
	runtime, err := m.runtime()
	if err != nil {
		return err
	}
	return runtime.RefreshMaterializations(ctx, modelID)
}

func (m *Manager) DataDir() string {
	return m.dataDir
}

func (m *Manager) Pages(dashboardID string) []dashboard.Page {
	runtime, err := m.runtime()
	if err != nil {
		return nil
	}
	return runtime.Pages(dashboardID)
}
