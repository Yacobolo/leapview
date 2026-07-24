package module

import (
	dashboardadapter "github.com/Yacobolo/leapview/internal/dashboard/analyticsduckdb"
	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
)

type RuntimeFactoryConfig = dashboardadapter.RuntimeFactoryConfig

func NewRuntimeFactory(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return dashboardadapter.NewRuntimeBuilder(config)
}
