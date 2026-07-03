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

type Lease interface {
	Runtime() Runtime
	DeploymentID() deployment.ID
	DuckLakeSnapshotID() int64
	Release()
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
	onDrained   func(deployment.ID, int64)

	activeDeployment deployment.ID
	activeDigest     string
	activeSnapshotID int64
	current          *managedRuntime
	retired          []*managedRuntime
}

type ManagerOptions struct {
	Repo        DeploymentRepository
	WorkspaceID deployment.WorkspaceID
	Environment deployment.Environment
	DataDir     string
	Factory     RuntimeFactory
	OnDrained   func(deployment.ID, int64)
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
		onDrained:   options.OnDrained,
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
	m.current = &managedRuntime{
		deploymentID: prepared.deploymentID,
		digest:       prepared.digest,
		runtime:      prepared.runtime,
		snapshotID:   prepared.snapshotID,
	}
	m.activeDeployment = prepared.deploymentID
	m.activeDigest = prepared.digest
	m.activeSnapshotID = prepared.snapshotID
	prepared.runtime = nil
	oldToClose := m.retireLocked(old)
	m.mu.Unlock()
	m.closeManaged(oldToClose)
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	current := m.current
	m.current = nil
	m.activeDeployment = ""
	m.activeDigest = ""
	m.activeSnapshotID = 0
	currentToClose := m.retireLocked(current)
	m.mu.Unlock()
	if currentToClose == nil {
		return nil
	}
	return m.closeManaged(currentToClose)
}

func (m *Manager) Active() (Runtime, error) {
	lease, err := m.Acquire()
	if err != nil {
		return nil, err
	}
	runtime := lease.Runtime()
	lease.Release()
	return runtime, nil
}

func (m *Manager) Acquire() (Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.current.closing {
		return nil, fmt.Errorf("no active LibreDash deployment")
	}
	m.current.refs++
	return &runtimeLease{manager: m, managed: m.current}, nil
}

func (m *Manager) LeasedSnapshots() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshots := map[int64]struct{}{}
	if m.current != nil && m.current.refs > 0 && m.current.snapshotID > 0 {
		snapshots[m.current.snapshotID] = struct{}{}
	}
	for _, runtime := range m.retired {
		if runtime.refs > 0 && runtime.snapshotID > 0 {
			snapshots[runtime.snapshotID] = struct{}{}
		}
	}
	return snapshotKeys(snapshots)
}

func (m *Manager) retireLocked(runtime *managedRuntime) *managedRuntime {
	if runtime == nil {
		return nil
	}
	runtime.closing = true
	if runtime.refs > 0 {
		m.retired = append(m.retired, runtime)
		return nil
	}
	return runtime
}

func (m *Manager) release(runtime *managedRuntime) {
	var drained *managedRuntime
	m.mu.Lock()
	if runtime != nil && runtime.refs > 0 {
		runtime.refs--
		if runtime.refs == 0 && runtime.closing {
			drained = runtime
			m.removeRetiredLocked(runtime)
		}
	}
	m.mu.Unlock()
	_ = m.closeManaged(drained)
}

func (m *Manager) removeRetiredLocked(runtime *managedRuntime) {
	for index, retired := range m.retired {
		if retired == runtime {
			m.retired = append(m.retired[:index], m.retired[index+1:]...)
			return
		}
	}
}

func (m *Manager) closeManaged(runtime *managedRuntime) error {
	if runtime == nil || runtime.runtime == nil {
		return nil
	}
	err := runtime.runtime.Close()
	if runtime.closing && m.onDrained != nil {
		m.onDrained(runtime.deploymentID, runtime.snapshotID)
	}
	return err
}

type managedRuntime struct {
	deploymentID deployment.ID
	digest       string
	runtime      Runtime
	snapshotID   int64
	refs         int
	closing      bool
}

type runtimeLease struct {
	manager *Manager
	managed *managedRuntime
	once    sync.Once
}

func (l *runtimeLease) Runtime() Runtime {
	if l == nil || l.managed == nil {
		return nil
	}
	return l.managed.runtime
}

func (l *runtimeLease) DeploymentID() deployment.ID {
	if l == nil || l.managed == nil {
		return ""
	}
	return l.managed.deploymentID
}

func (l *runtimeLease) DuckLakeSnapshotID() int64 {
	if l == nil || l.managed == nil {
		return 0
	}
	return l.managed.snapshotID
}

func (l *runtimeLease) Release() {
	if l == nil || l.manager == nil || l.managed == nil {
		return
	}
	l.once.Do(func() {
		l.manager.release(l.managed)
	})
}

func snapshotKeys(values map[int64]struct{}) []int64 {
	if len(values) == 0 {
		return nil
	}
	keys := make([]int64, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	return keys
}
