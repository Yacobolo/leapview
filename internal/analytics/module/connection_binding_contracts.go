package module

import (
	"context"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

type ConnectionAdministrationPermission = connectionbinding.AdministrationPermission
type ConnectionTargetBinding = connectionbinding.TargetBinding
type ConnectionBindingDependency = connectionbinding.BindingDependency

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
	Authorize    ConnectionAdministrationAuthorizer
	Dependencies ConnectionDependencyInspector
	Pools        connectionbinding.AdministrationPoolDirectory
	Now          func() time.Time
}
