package runtimehost

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Yacobolo/libredash/internal/deployment"
)

type DeploymentRepository interface {
	ActiveArtifact(ctx context.Context, workspaceID deployment.WorkspaceID, environment deployment.Environment) (deployment.Deployment, deployment.Artifact, error)
	ByID(ctx context.Context, id deployment.ID) (deployment.Deployment, error)
	ArtifactByDeployment(ctx context.Context, deploymentID deployment.ID) (deployment.Artifact, error)
	RecordDuckLakeSnapshot(ctx context.Context, deploymentID deployment.ID, snapshotID int64) error
}

type Runtime interface {
	Close() error
}

type RuntimeSnapshot interface {
	DuckLakeSnapshotID() int64
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
	environment deployment.Environment
	dataDir     string
	factory     RuntimeFactory

	activeDeployment deployment.ID
	activeDigest     string
	activeSnapshotID int64
	current          Runtime
}

type ManagerOptions struct {
	Repo        DeploymentRepository
	WorkspaceID deployment.WorkspaceID
	Environment deployment.Environment
	DataDir     string
	Factory     RuntimeFactory
}

type Prepared struct {
	deploymentID deployment.ID
	digest       string
	runtime      Runtime
	noChange     bool
	snapshotID   int64
}

func (p *Prepared) Close() error {
	if p == nil || p.runtime == nil {
		return nil
	}
	return p.runtime.Close()
}

func (p *Prepared) DuckLakeSnapshotID() int64 {
	if p == nil {
		return 0
	}
	return p.snapshotID
}

func NewManagerWithFactory(options ManagerOptions) *Manager {
	return &Manager{
		repo:        options.Repo,
		workspaceID: options.WorkspaceID,
		environment: deployment.NormalizeEnvironment(options.Environment),
		dataDir:     options.DataDir,
		factory:     options.Factory,
	}
}

func (m *Manager) Reload(ctx context.Context) error {
	current, artifact, err := m.repo.ActiveArtifact(ctx, m.workspaceID, m.environment)
	if err != nil {
		if errors.Is(err, deployment.ErrNotFound) {
			return m.Close()
		}
		return err
	}
	prepared, err := m.prepare(ctx, current, artifact)
	if err != nil {
		return err
	}
	if current.DuckLakeSnapshotID == 0 && prepared.DuckLakeSnapshotID() > 0 {
		if err := m.repo.RecordDuckLakeSnapshot(ctx, current.ID, prepared.DuckLakeSnapshotID()); err != nil {
			_ = prepared.Close()
			return err
		}
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
	if m.current != nil && m.activeDeployment == current.ID && m.activeDigest == artifact.Digest && m.activeSnapshotID == current.DuckLakeSnapshotID {
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
	var snapshotID int64
	if snapshot, ok := runtime.(RuntimeSnapshot); ok {
		snapshotID = snapshot.DuckLakeSnapshotID()
	}
	if snapshotID == 0 {
		snapshotID = current.DuckLakeSnapshotID
	}
	return &Prepared{deploymentID: current.ID, digest: artifact.Digest, runtime: runtime, snapshotID: snapshotID}, nil
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
	m.activeSnapshotID = prepared.snapshotID
	prepared.runtime = nil
	m.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	current := m.current
	m.current = nil
	m.activeDeployment = ""
	m.activeDigest = ""
	m.activeSnapshotID = 0
	m.mu.Unlock()
	if current == nil {
		return nil
	}
	return current.Close()
}

func (m *Manager) Active() (Runtime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return nil, fmt.Errorf("no active LibreDash deployment")
	}
	return m.current, nil
}
