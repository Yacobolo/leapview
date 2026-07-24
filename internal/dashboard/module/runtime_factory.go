package module

import (
	dashboardanalytics "github.com/Yacobolo/leapview/internal/dashboard/analyticsruntime"
	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
)

type RuntimeFactoryConfig = dashboardanalytics.RuntimeFactoryConfig

func NewRuntimeFactory(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return dashboardanalytics.NewRuntimeBuilder(config)
}
