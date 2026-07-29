package module

import (
	"context"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

type ConnectionAdministrationPermission = connectionbinding.AdministrationPermission
type ConnectionTargetBinding = connectionbinding.TargetBinding
type ConnectionBindingDependency = connectionbinding.BindingDependency
type ConnectionRotationAuditEvent = connectionbinding.RotationAuditEvent
type ConnectionRotationAuditRecorder = connectionbinding.RotationAuditRecorder

const (
	PermissionManageConnectionMetadata = connectionbinding.PermissionManageConnectionMetadata
	PermissionTestConnection           = connectionbinding.PermissionTestConnection
	PermissionViewConnectionHealth     = connectionbinding.PermissionViewConnectionHealth
)

var ErrConnectionAdministrationUnavailable = connectionbinding.ErrProviderUnavailable
var ErrConnectionBindingUnauthorized = connectionbinding.ErrUnauthorizedBinding

type ConnectionDependencyInspector interface {
	Dependents(context.Context, ConnectionTargetBinding) ([]ConnectionBindingDependency, error)
}

type ConnectionAdministrationAuthorizer func(
	context.Context,
	string,
	ConnectionAdministrationPermission,
	ConnectionTargetBinding,
) error

type ConnectionAdministrationConfig struct {
	Authorize      ConnectionAdministrationAuthorizer
	Dependencies   ConnectionDependencyInspector
	Pools          connectionbinding.AdministrationPoolDirectory
	Now            func() time.Time
	RefreshTimeout time.Duration
	MaxConcurrent  int
	Audit          ConnectionRotationAuditRecorder
}
