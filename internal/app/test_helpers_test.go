package app

import (
	"context"

	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	"github.com/Yacobolo/leapview/internal/runtimehost"
)

func assembleRuntime(metrics QueryMetrics, options assemblyConfig) *runtimeRouter {
	server, err := assembleRuntimeChecked(context.Background(), metrics, options)
	if err != nil {
		panic(err)
	}
	return server
}

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
