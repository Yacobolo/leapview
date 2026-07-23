package app

import (
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	"github.com/Yacobolo/leapview/internal/runtimehost"
)

func NewRuntimeMetrics(provider runtimehost.Provider, workspaceID string) QueryMetrics {
	return dashboardmodule.NewRuntimeMetrics(provider, workspaceID)
}

func NewDynamicRuntimeMetrics(defaultWorkspaceID string, factory func(string) runtimehost.Provider) QueryMetrics {
	return dashboardmodule.NewDynamicRuntimeMetrics(defaultWorkspaceID, factory)
}

func NewMultiWorkspaceMetrics(defaultWorkspaceID string, workspaces map[string]QueryMetrics) QueryMetrics {
	return dashboardmodule.NewMultiWorkspaceMetrics(defaultWorkspaceID, workspaces)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
