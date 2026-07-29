package module

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

func TestConnectionAdministrationComposesTargetOwnedValidatedPoolDirectory(t *testing.T) {
	now := time.Date(2026, 7, 29, 20, 0, 0, 0, time.UTC)
	binding := modulePoolBinding(t, now)
	repository := &moduleBindingCatalog{binding: binding}
	resolver := &moduleCredentialResolver{snapshot: modulePoolSnapshot(t, now)}
	factory := &moduleRuntimePoolFactory{}
	module := &Module{
		connectionBindings: repository,
		targetResolvers:    connectionbinding.ResolverSet{Infisical: resolver},
		targetID:           binding.TargetID,
		targetEnvironment:  binding.Scope.Environment,
		targetClass:        connectionbinding.TargetProduction,
		connectionFactory:  factory,
	}
	administration, err := module.NewConnectionAdministration(ConnectionAdministrationConfig{
		Authorize: func(context.Context, string, ConnectionAdministrationPermission, ConnectionTargetBinding) error {
			return nil
		},
		Dependencies:   moduleDependencyInspector{},
		Now:            func() time.Time { return now },
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	health, err := administration.Test(context.Background(), "operator-1", connectionbinding.BindingKey{
		Scope: binding.Scope, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || factory.calls != 1 || health.Health != connectionbinding.HealthHealthy ||
		health.ValidatedVersion != "secret:v2" || !health.HasActivePool {
		t.Fatalf("resolver=%d factory=%d health=%#v", resolver.calls, factory.calls, health)
	}
	if module.connectionPools == nil {
		t.Fatal("module did not retain the target-owned pool directory")
	}
	runtimeBindings, err := module.NewRuntimeBindingLeaser(RuntimeBindingLeaserConfig{
		Authorize: func(context.Context, string, ConnectionTargetBinding) error {
			return nil
		},
		Now:            func() time.Time { return now },
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	leases, err := runtimeBindings.Acquire(t.Context(), RuntimeBindingRequest{
		Actor: "principal:author-1", Scope: binding.Scope, TargetID: binding.TargetID,
		Requirements: []connectionbinding.Requirement{{
			LogicalConnectionID: binding.LogicalConnectionID,
			ConnectorKind:       binding.ConnectorKind,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence := leases.Evidence()
	if len(evidence) != 1 || evidence[0].ValidatedVersion != "secret:v2" ||
		resolver.calls != 1 || factory.calls != 1 {
		t.Fatalf(
			"runtime evidence=%#v resolver=%d factory=%d, want reused validated generation",
			evidence,
			resolver.calls,
			factory.calls,
		)
	}
	leases.Release()
	if err := module.Close(); err != nil {
		t.Fatal(err)
	}
	if !factory.pool.closed {
		t.Fatal("module close did not retire the active connection pool")
	}
}

func TestConnectionAdministrationRejectsBindingsForAnotherTarget(t *testing.T) {
	now := time.Date(2026, 7, 29, 20, 0, 0, 0, time.UTC)
	binding := modulePoolBinding(t, now)
	repository := &moduleBindingCatalog{binding: binding}
	module := &Module{
		connectionBindings: repository,
		targetResolvers: connectionbinding.ResolverSet{
			Infisical: &moduleCredentialResolver{snapshot: modulePoolSnapshot(t, now)},
		},
		targetID:          "this-target",
		targetEnvironment: binding.Scope.Environment,
		targetClass:       connectionbinding.TargetProduction,
		connectionFactory: &moduleRuntimePoolFactory{},
	}
	administration, err := module.NewConnectionAdministration(ConnectionAdministrationConfig{
		Authorize: func(context.Context, string, ConnectionAdministrationPermission, ConnectionTargetBinding) error {
			return nil
		},
		Dependencies:   moduleDependencyInspector{},
		Now:            func() time.Time { return now },
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = administration.Get(context.Background(), "operator-1", connectionbinding.BindingKey{
		Scope: binding.Scope, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
	})
	if !errors.Is(err, connectionbinding.ErrUnauthorizedBinding) {
		t.Fatalf("cross-target Get() error = %v", err)
	}
}

func TestConnectionAdministrationUsesExplicitEnvironmentResolverOnlyForDevelopmentTarget(t *testing.T) {
	now := time.Date(2026, 7, 29, 20, 0, 0, 0, time.UTC)
	binding := modulePoolBinding(t, now)
	binding.TargetID = "lvinst_local"
	binding.Scope.Environment = "dev"
	binding.CredentialReference = connectionbinding.CredentialReference{
		ProjectID: "lvinst_local", Environment: "dev",
		SecretPath: "/", SecretKey: "LEAPVIEW_DEV_CONNECTION_WAREHOUSE",
	}
	repository := &moduleBindingCatalog{binding: binding}
	resolver := &moduleCredentialResolver{snapshot: modulePoolSnapshot(t, now)}
	module := &Module{
		connectionBindings: repository,
		targetResolvers:    connectionbinding.ResolverSet{Environment: resolver},
		targetID:           binding.TargetID,
		targetEnvironment:  binding.Scope.Environment,
		targetClass:        connectionbinding.TargetDevelopment,
		connectionFactory:  &moduleRuntimePoolFactory{},
	}
	administration, err := module.NewConnectionAdministration(ConnectionAdministrationConfig{
		Authorize: func(context.Context, string, ConnectionAdministrationPermission, ConnectionTargetBinding) error {
			return nil
		},
		Dependencies:   moduleDependencyInspector{},
		Now:            func() time.Time { return now },
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = administration.Test(context.Background(), "operator-1", connectionbinding.BindingKey{
		Scope: binding.Scope, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 {
		t.Fatalf("development environment resolver calls = %d", resolver.calls)
	}
}

func modulePoolBinding(t *testing.T, now time.Time) connectionbinding.TargetBinding {
	t.Helper()
	binding, err := connectionbinding.NewTargetBinding(connectionbinding.TargetBindingInput{
		ID: "binding_prod_warehouse", TargetID: "lvinst_prod", LogicalConnectionID: "warehouse",
		ConnectorKind: "postgres", AuthenticationMode: connectionbinding.AuthenticationExternalBundle,
		Scope: connectionbinding.BindingScope{WorkspaceID: "sales", Environment: "prod"},
		Endpoint: connectionbinding.EndpointConfig{
			Host: "warehouse.internal", Port: 5432, Database: "analytics",
			SourceIdentity: "runtime", TLSMode: "verify-full",
		},
		CredentialReference: connectionbinding.CredentialReference{
			ProjectID: "project-1", Environment: "prod", SecretPath: "/leapview/sales", SecretKey: "warehouse",
		},
		Enabled: true, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func modulePoolSnapshot(t *testing.T, now time.Time) connectionbinding.CredentialSnapshot {
	t.Helper()
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"password": "source-secret"}, "secret:v2", now, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

type moduleBindingCatalog struct {
	binding connectionbinding.TargetBinding
}

func (catalog *moduleBindingCatalog) Create(_ context.Context, binding connectionbinding.TargetBinding) error {
	catalog.binding = binding
	return nil
}

func (catalog *moduleBindingCatalog) Binding(
	context.Context,
	connectionbinding.BindingScope,
	string,
	connectionbinding.LogicalConnectionID,
) (connectionbinding.TargetBinding, error) {
	return catalog.binding, nil
}

func (catalog *moduleBindingCatalog) List(
	context.Context,
	connectionbinding.BindingScope,
	string,
) ([]connectionbinding.TargetBinding, error) {
	return []connectionbinding.TargetBinding{catalog.binding}, nil
}

func (catalog *moduleBindingCatalog) Save(
	_ context.Context,
	binding connectionbinding.TargetBinding,
	expected int64,
) (connectionbinding.TargetBinding, error) {
	if catalog.binding.Revision != expected {
		return connectionbinding.TargetBinding{}, connectionbinding.ErrIncompatibleBinding
	}
	catalog.binding = binding
	return binding, nil
}

type moduleCredentialResolver struct {
	snapshot connectionbinding.CredentialSnapshot
	calls    int
}

func (resolver *moduleCredentialResolver) Resolve(
	context.Context,
	connectionbinding.CredentialReference,
) (connectionbinding.CredentialSnapshot, error) {
	resolver.calls++
	return resolver.snapshot, nil
}

type moduleRuntimePoolFactory struct {
	calls int
	pool  *moduleRuntimePool
}

func (factory *moduleRuntimePoolFactory) Prepare(
	context.Context,
	connectionbinding.TargetBinding,
	connectionbinding.CredentialSnapshot,
) (connectionbinding.RuntimePool, error) {
	factory.calls++
	factory.pool = &moduleRuntimePool{}
	return factory.pool, nil
}

type moduleRuntimePool struct {
	closed bool
}

func (*moduleRuntimePool) HealthCheck(context.Context) error { return nil }
func (pool *moduleRuntimePool) Close() error {
	pool.closed = true
	return nil
}

type moduleDependencyInspector struct{}

func (moduleDependencyInspector) Dependents(
	context.Context,
	ConnectionTargetBinding,
) ([]ConnectionBindingDependency, error) {
	return nil, nil
}
