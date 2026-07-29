package connectionbinding

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdministrationRequiresDependencyPlanConfirmationForConfigurationChanges(t *testing.T) {
	now := time.Date(2026, 7, 29, 19, 0, 0, 0, time.UTC)
	binding := validTargetBinding(t)
	repository := &administrationRepository{binding: binding}
	service, err := NewAdministration(AdministrationConfig{
		Repository: repository,
		Authorize:  allowAdministration,
		Dependencies: staticDependencyInspector{dependencies: []BindingDependency{
			{Kind: "candidate", ID: "candidate-1", Label: "Author preview"},
			{Kind: "serving_state", ID: "state-1", Label: "Active sales"},
		}},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	key := BindingKey{
		Scope: binding.Scope, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
	}
	configuration := binding.Configuration()
	configuration.Endpoint.Host = "warehouse-next.internal"
	plan, err := service.PlanConfigurationChange(context.Background(), "operator-1", key, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresConfirmation || plan.ConfirmationToken == "" || len(plan.Dependencies) != 2 ||
		plan.ExpectedRevision != binding.Revision {
		t.Fatalf("change plan = %#v", plan)
	}
	if _, err := service.UpdateConfiguration(context.Background(), UpdateConfigurationRequest{
		ActorID: "operator-1", Key: key, Configuration: configuration,
		ExpectedRevision: binding.Revision,
	}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("unconfirmed update error = %v", err)
	}
	updated, err := service.UpdateConfiguration(context.Background(), UpdateConfigurationRequest{
		ActorID: "operator-1", Key: key, Configuration: configuration,
		ExpectedRevision: binding.Revision, ConfirmationToken: plan.ConfirmationToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Endpoint.Host != "warehouse-next.internal" || repository.saves != 1 {
		t.Fatalf("updated=%#v saves=%d", updated, repository.saves)
	}
}

func TestAdministrationSeparatesMetadataAndRefreshAuthorization(t *testing.T) {
	binding := validTargetBinding(t)
	repository := &administrationRepository{binding: binding}
	pool := &administrationPool{}
	service, err := NewAdministration(AdministrationConfig{
		Repository: repository,
		Authorize: func(_ context.Context, actor string, permission AdministrationPermission, _ TargetBinding) error {
			if actor != "metadata-operator" || permission != PermissionManageConnectionMetadata {
				return ErrUnauthorizedBinding
			}
			return nil
		},
		Dependencies: staticDependencyInspector{},
		Pools:        staticPoolDirectory{pool: pool},
		Now:          time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := BindingKey{
		Scope: binding.Scope, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
	}
	if err := service.RefreshNow(context.Background(), "metadata-operator", key); !errors.Is(err, ErrUnauthorizedBinding) {
		t.Fatalf("RefreshNow() error = %v", err)
	}
	if pool.refreshes != 0 {
		t.Fatalf("unauthorized refreshes = %d", pool.refreshes)
	}
}

type administrationRepository struct {
	binding TargetBinding
	saves   int
}

func (repository *administrationRepository) Create(_ context.Context, binding TargetBinding) error {
	repository.binding = binding
	return nil
}

func (repository *administrationRepository) Binding(
	context.Context,
	BindingScope,
	string,
	LogicalConnectionID,
) (TargetBinding, error) {
	return repository.binding, nil
}

func (repository *administrationRepository) Save(
	_ context.Context,
	binding TargetBinding,
	expectedRevision int64,
) (TargetBinding, error) {
	if repository.binding.Revision != expectedRevision {
		return TargetBinding{}, ErrIncompatibleBinding
	}
	repository.binding = binding
	repository.saves++
	return binding, nil
}

func allowAdministration(context.Context, string, AdministrationPermission, TargetBinding) error {
	return nil
}

type staticDependencyInspector struct {
	dependencies []BindingDependency
}

func (inspector staticDependencyInspector) Dependents(context.Context, TargetBinding) ([]BindingDependency, error) {
	return append([]BindingDependency(nil), inspector.dependencies...), nil
}

type administrationPool struct {
	refreshes int
}

func (pool *administrationPool) Refresh(context.Context, RefreshRequest) error {
	pool.refreshes++
	return nil
}

func (pool *administrationPool) Disable(context.Context, time.Time) error { return nil }
func (pool *administrationPool) HealthStatus() BindingHealthStatus        { return BindingHealthStatus{} }

type staticPoolDirectory struct {
	pool AdministrationPool
}

func (directory staticPoolDirectory) Pool(string) (AdministrationPool, error) {
	if directory.pool == nil {
		return nil, ErrBindingNotFound
	}
	return directory.pool, nil
}
