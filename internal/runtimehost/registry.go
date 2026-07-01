package runtimehost

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Yacobolo/libredash/internal/deployment"
)

type RegistryOptions struct {
	Repo         DeploymentRepository
	WorkspaceIDs []deployment.WorkspaceID
	Environment  deployment.Environment
	DataDir      string
	Factory      RuntimeFactory
}

type Registry struct {
	mu          sync.RWMutex
	repo        DeploymentRepository
	environment deployment.Environment
	dataDir     string
	factory     RuntimeFactory
	managers    map[deployment.WorkspaceID]*Manager
}

type RegistryPrepared struct {
	workspaceID deployment.WorkspaceID
	manager     *Manager
	prepared    deployment.PreparedRuntime
}

func (p *RegistryPrepared) Close() error {
	if p == nil || p.prepared == nil {
		return nil
	}
	return p.prepared.Close()
}

type WorkspaceProvider struct {
	registry    *Registry
	workspaceID deployment.WorkspaceID
}

func NewRegistryWithFactory(options RegistryOptions) *Registry {
	registry := &Registry{
		repo:        options.Repo,
		environment: deployment.NormalizeEnvironment(options.Environment),
		dataDir:     options.DataDir,
		factory:     options.Factory,
		managers:    map[deployment.WorkspaceID]*Manager{},
	}
	for _, workspaceID := range options.WorkspaceIDs {
		registry.managerForWorkspace(workspaceID)
	}
	return registry
}

func (r *Registry) Reload(ctx context.Context) error {
	for _, workspaceID := range r.workspaceIDs() {
		manager := r.managerForWorkspace(workspaceID)
		if err := manager.Reload(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error) {
	current, err := r.repo.ByID(ctx, deployment.ID(deploymentID))
	if err != nil {
		return nil, err
	}
	if deployment.NormalizeEnvironment(current.Environment) != r.environment {
		return nil, fmt.Errorf("deployment %s environment = %q, want %q", deploymentID, current.Environment, r.environment)
	}
	manager := r.managerForWorkspace(current.WorkspaceID)
	prepared, err := manager.PrepareDeployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	return &RegistryPrepared{workspaceID: current.WorkspaceID, manager: manager, prepared: prepared}, nil
}

func (r *Registry) CommitPrepared(candidate deployment.PreparedRuntime) error {
	prepared, ok := candidate.(*RegistryPrepared)
	if !ok {
		return fmt.Errorf("prepared runtime belongs to a different host")
	}
	if prepared == nil || prepared.manager == nil || prepared.prepared == nil {
		return fmt.Errorf("prepared runtime is nil")
	}
	return prepared.manager.CommitPrepared(prepared.prepared)
}

func (r *Registry) Close() error {
	var first error
	for _, workspaceID := range r.workspaceIDs() {
		if err := r.managerForWorkspace(workspaceID).Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *Registry) ActiveForWorkspace(workspaceID deployment.WorkspaceID) (Runtime, error) {
	r.mu.RLock()
	manager := r.managers[workspaceID]
	r.mu.RUnlock()
	if manager == nil {
		return nil, fmt.Errorf("no active LibreDash deployment")
	}
	return manager.Active()
}

func (r *Registry) ProviderForWorkspace(workspaceID deployment.WorkspaceID) *WorkspaceProvider {
	r.managerForWorkspace(workspaceID)
	return &WorkspaceProvider{registry: r, workspaceID: workspaceID}
}

func (p *WorkspaceProvider) Active() (Runtime, error) {
	if p == nil || p.registry == nil {
		return nil, fmt.Errorf("runtime provider is not configured")
	}
	return p.registry.ActiveForWorkspace(p.workspaceID)
}

func (r *Registry) managerForWorkspace(workspaceID deployment.WorkspaceID) *Manager {
	r.mu.Lock()
	defer r.mu.Unlock()
	if manager := r.managers[workspaceID]; manager != nil {
		return manager
	}
	manager := NewManagerWithFactory(ManagerOptions{
		Repo:        r.repo,
		WorkspaceID: workspaceID,
		Environment: r.environment,
		DataDir:     r.dataDir,
		Factory:     r.factory,
	})
	r.managers[workspaceID] = manager
	return manager
}

func (r *Registry) workspaceIDs() []deployment.WorkspaceID {
	r.mu.RLock()
	ids := make([]deployment.WorkspaceID, 0, len(r.managers))
	for id := range r.managers {
		ids = append(ids, id)
	}
	r.mu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
