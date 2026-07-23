package module

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Yacobolo/leapview/internal/runtimehost"
	"github.com/Yacobolo/leapview/internal/servingstate"
)

type Config struct {
	States           runtimehost.ServingStateRepository
	WorkspaceIDs     []servingstate.WorkspaceID
	Environment      servingstate.Environment
	Factory          runtimehost.RuntimeFactory
	ManagedData      runtimehost.ManagedDataResolver
	Logger           *slog.Logger
	OnDrained        func(servingstate.ID, int64, []int64)
	OnCleanupFailure func(runtimehost.CleanupFailure)
}

type Module struct{ registry *runtimehost.Registry }

func Build(ctx context.Context, config Config) (*Module, error) {
	if config.States == nil || config.Factory == nil {
		return nil, errors.New("serving-state repository and runtime factory are required")
	}
	var registry *runtimehost.Registry
	registry = runtimehost.NewRegistryWithFactory(runtimehost.RegistryOptions{
		Repo: config.States, WorkspaceIDs: config.WorkspaceIDs, Environment: config.Environment,
		Factory: config.Factory, ManagedData: config.ManagedData, Logger: config.Logger,
		OnCleanupFailure: config.OnCleanupFailure,
		OnDrained: func(id servingstate.ID, snapshot int64) {
			if config.OnDrained != nil {
				config.OnDrained(id, snapshot, registry.LeasedSnapshots())
			}
		},
	})
	if err := registry.Reload(ctx); err != nil {
		_ = registry.Close()
		return nil, err
	}
	return &Module{registry: registry}, nil
}

func (m *Module) Reload(ctx context.Context) error { return m.registry.Reload(ctx) }
func (m *Module) PrepareServingState(ctx context.Context, id string) (servingstate.PreparedRuntime, error) {
	return m.registry.PrepareServingState(ctx, id)
}
func (m *Module) PrepareServingStateCandidates(ctx context.Context, inputs []runtimehost.ServingStateCandidate) (*runtimehost.PreparedSet, error) {
	return m.registry.PrepareServingStateCandidates(ctx, inputs)
}
func (m *Module) ActivatePrepared(candidate servingstate.PreparedRuntime, activate func() error) error {
	return m.registry.ActivatePrepared(candidate, activate)
}
func (m *Module) ActivatePreparedSet(set *runtimehost.PreparedSet, activate func() error) error {
	return m.registry.ActivatePreparedSet(set, activate)
}
func (m *Module) ProviderForWorkspace(id servingstate.WorkspaceID) runtimehost.Provider {
	return m.registry.ProviderForWorkspace(id)
}
func (m *Module) LeasedSnapshots() []int64 { return m.registry.LeasedSnapshots() }
func (m *Module) Close() error             { return m.registry.Close() }
