package connectionbinding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	platformsecret "github.com/flidai/leapview/internal/platform/security/secret"
)

var ErrConfirmationRequired = errors.New("connection binding change confirmation required")

type AdministrationPermission string

const (
	PermissionManageConnectionMetadata AdministrationPermission = "connection.metadata.manage"
	PermissionTestConnection           AdministrationPermission = "connection.test"
	PermissionViewConnectionHealth     AdministrationPermission = "connection.health.view"
)

type BindingKey struct {
	Scope               BindingScope
	TargetID            string
	LogicalConnectionID LogicalConnectionID
}

type BindingDependency struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type DependencyInspector interface {
	Dependents(context.Context, TargetBinding) ([]BindingDependency, error)
}

type AdministrationAuthorizer func(context.Context, string, AdministrationPermission, TargetBinding) error

type AdministrationPool interface {
	Refresh(context.Context, RefreshRequest) error
	Disable(context.Context, time.Time) error
	HealthStatus() BindingHealthStatus
}

type AdministrationPoolDirectory interface {
	Pool(bindingID string) (AdministrationPool, error)
}

type AdministrationConfig struct {
	Repository   Repository
	Authorize    AdministrationAuthorizer
	Dependencies DependencyInspector
	Pools        AdministrationPoolDirectory
	Now          func() time.Time
}

type Administration struct {
	repository   Repository
	authorize    AdministrationAuthorizer
	dependencies DependencyInspector
	pools        AdministrationPoolDirectory
	now          func() time.Time
}

func NewAdministration(config AdministrationConfig) (*Administration, error) {
	if config.Repository == nil || config.Authorize == nil || config.Dependencies == nil || config.Now == nil {
		return nil, fmt.Errorf("%w: binding repository, authorizer, dependency inspector, and clock are required", ErrInvalidBinding)
	}
	return &Administration{
		repository: config.Repository, authorize: config.Authorize,
		dependencies: config.Dependencies, pools: config.Pools, now: config.Now,
	}, nil
}

type BindingChangePlan struct {
	BindingID            string              `json:"bindingId"`
	ExpectedRevision     int64               `json:"expectedRevision"`
	RequiresConfirmation bool                `json:"requiresConfirmation"`
	ConfirmationToken    string              `json:"confirmationToken,omitempty"`
	Dependencies         []BindingDependency `json:"dependencies,omitempty"`
}

func (service *Administration) Create(
	ctx context.Context,
	actorID string,
	input TargetBindingInput,
) (TargetBinding, error) {
	if service == nil {
		return TargetBinding{}, ErrProviderUnavailable
	}
	input.Now = service.now().UTC()
	binding, err := NewTargetBinding(input)
	if err != nil {
		return TargetBinding{}, err
	}
	if err := service.authorize(
		ctx, strings.TrimSpace(actorID), PermissionManageConnectionMetadata, binding,
	); err != nil {
		return TargetBinding{}, ErrUnauthorizedBinding
	}
	if err := service.repository.Create(ctx, binding); err != nil {
		return TargetBinding{}, err
	}
	return binding, nil
}

func (service *Administration) PlanConfigurationChange(
	ctx context.Context,
	actorID string,
	key BindingKey,
	configuration TargetBindingConfiguration,
) (BindingChangePlan, error) {
	binding, err := service.binding(ctx, key)
	if err != nil {
		return BindingChangePlan{}, err
	}
	if err := service.authorize(ctx, strings.TrimSpace(actorID), PermissionManageConnectionMetadata, binding); err != nil {
		return BindingChangePlan{}, ErrUnauthorizedBinding
	}
	updated, err := binding.UpdateConfiguration(configuration, service.now().UTC())
	if err != nil {
		return BindingChangePlan{}, err
	}
	plan := BindingChangePlan{BindingID: binding.ID, ExpectedRevision: binding.Revision}
	if updated.Revision == binding.Revision {
		return plan, nil
	}
	dependencies, err := service.dependencies.Dependents(ctx, binding)
	if err != nil {
		return BindingChangePlan{}, err
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Kind != dependencies[j].Kind {
			return dependencies[i].Kind < dependencies[j].Kind
		}
		return dependencies[i].ID < dependencies[j].ID
	})
	plan.Dependencies = dependencies
	plan.RequiresConfirmation = len(dependencies) > 0
	if plan.RequiresConfirmation {
		plan.ConfirmationToken = changeConfirmationToken(binding, updated.Configuration(), dependencies)
	}
	return plan, nil
}

type UpdateConfigurationRequest struct {
	ActorID           string
	Key               BindingKey
	Configuration     TargetBindingConfiguration
	ExpectedRevision  int64
	ConfirmationToken string
}

func (service *Administration) UpdateConfiguration(
	ctx context.Context,
	request UpdateConfigurationRequest,
) (TargetBinding, error) {
	binding, err := service.binding(ctx, request.Key)
	if err != nil {
		return TargetBinding{}, err
	}
	if request.ExpectedRevision != binding.Revision {
		return TargetBinding{}, ErrIncompatibleBinding
	}
	if err := service.authorize(
		ctx, strings.TrimSpace(request.ActorID), PermissionManageConnectionMetadata, binding,
	); err != nil {
		return TargetBinding{}, ErrUnauthorizedBinding
	}
	updated, err := binding.UpdateConfiguration(request.Configuration, service.now().UTC())
	if err != nil {
		return TargetBinding{}, err
	}
	if updated.Revision == binding.Revision {
		return binding, nil
	}
	dependencies, err := service.dependencies.Dependents(ctx, binding)
	if err != nil {
		return TargetBinding{}, err
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Kind != dependencies[j].Kind {
			return dependencies[i].Kind < dependencies[j].Kind
		}
		return dependencies[i].ID < dependencies[j].ID
	})
	if len(dependencies) > 0 {
		expected := changeConfirmationToken(binding, updated.Configuration(), dependencies)
		if !platformsecret.Equal(strings.TrimSpace(request.ConfirmationToken), expected) {
			return TargetBinding{}, ErrConfirmationRequired
		}
	}
	return service.repository.Save(ctx, updated, binding.Revision)
}

func (service *Administration) RefreshNow(ctx context.Context, actorID string, key BindingKey) error {
	binding, err := service.binding(ctx, key)
	if err != nil {
		return err
	}
	if err := service.authorize(ctx, strings.TrimSpace(actorID), PermissionTestConnection, binding); err != nil {
		return ErrUnauthorizedBinding
	}
	if service.pools == nil {
		return ErrProviderUnavailable
	}
	pool, err := service.pools.Pool(binding.ID)
	if err != nil {
		return err
	}
	return pool.Refresh(ctx, RefreshRequest{
		Actor: "principal:" + strings.TrimSpace(actorID), Operation: RefreshRequested,
	})
}

func (service *Administration) Health(
	ctx context.Context,
	actorID string,
	key BindingKey,
) (BindingHealthStatus, error) {
	binding, err := service.binding(ctx, key)
	if err != nil {
		return BindingHealthStatus{}, err
	}
	if err := service.authorize(
		ctx, strings.TrimSpace(actorID), PermissionViewConnectionHealth, binding,
	); err != nil {
		return BindingHealthStatus{}, ErrUnauthorizedBinding
	}
	if service.pools == nil {
		return bindingHealthWithoutPool(binding), nil
	}
	pool, err := service.pools.Pool(binding.ID)
	if errors.Is(err, ErrBindingNotFound) {
		return bindingHealthWithoutPool(binding), nil
	}
	if err != nil {
		return BindingHealthStatus{}, err
	}
	return pool.HealthStatus(), nil
}

func (service *Administration) Disable(
	ctx context.Context,
	actorID string,
	key BindingKey,
) (TargetBinding, error) {
	binding, err := service.binding(ctx, key)
	if err != nil {
		return TargetBinding{}, err
	}
	if err := service.authorize(
		ctx, strings.TrimSpace(actorID), PermissionManageConnectionMetadata, binding,
	); err != nil {
		return TargetBinding{}, ErrUnauthorizedBinding
	}
	now := service.now().UTC()
	if service.pools != nil {
		pool, poolErr := service.pools.Pool(binding.ID)
		if poolErr == nil {
			if err := pool.Disable(ctx, now); err != nil {
				return TargetBinding{}, err
			}
			return service.binding(ctx, key)
		}
		if !errors.Is(poolErr, ErrBindingNotFound) {
			return TargetBinding{}, poolErr
		}
	}
	disabled, err := binding.Disable(now)
	if err != nil || disabled.Revision == binding.Revision {
		return disabled, err
	}
	return service.repository.Save(ctx, disabled, binding.Revision)
}

func (service *Administration) Enable(
	ctx context.Context,
	actorID string,
	key BindingKey,
) (TargetBinding, error) {
	binding, err := service.binding(ctx, key)
	if err != nil {
		return TargetBinding{}, err
	}
	if err := service.authorize(
		ctx, strings.TrimSpace(actorID), PermissionManageConnectionMetadata, binding,
	); err != nil {
		return TargetBinding{}, ErrUnauthorizedBinding
	}
	enabled, err := binding.Enable(service.now().UTC())
	if err != nil || enabled.Revision == binding.Revision {
		return enabled, err
	}
	return service.repository.Save(ctx, enabled, binding.Revision)
}

func (service *Administration) binding(ctx context.Context, key BindingKey) (TargetBinding, error) {
	if service == nil {
		return TargetBinding{}, ErrProviderUnavailable
	}
	return service.repository.Binding(ctx, key.Scope, strings.TrimSpace(key.TargetID), key.LogicalConnectionID)
}

func bindingHealthWithoutPool(binding TargetBinding) BindingHealthStatus {
	return BindingHealthStatus{
		BindingID: binding.ID, TargetID: binding.TargetID,
		LogicalConnection: binding.LogicalConnectionID, ConnectorKind: binding.ConnectorKind,
		Scope: binding.Scope, BindingRevision: binding.Revision,
		ValidatedVersion: binding.ValidatedVersion, Health: binding.Health,
		Reason: binding.HealthReason, LastValidatedAt: binding.LastValidatedAt,
	}
}

func changeConfirmationToken(
	binding TargetBinding,
	configuration TargetBindingConfiguration,
	dependencies []BindingDependency,
) string {
	payload := struct {
		BindingID     string
		Revision      int64
		Configuration TargetBindingConfiguration
		Dependencies  []BindingDependency
	}{
		BindingID: binding.ID, Revision: binding.Revision,
		Configuration: configuration, Dependencies: dependencies,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}
