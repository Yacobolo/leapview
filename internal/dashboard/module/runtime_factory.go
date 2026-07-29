package module

import (
	dashboardanalytics "github.com/flidai/leapview/internal/dashboard/analyticsruntime"
	dashboardruntimefactory "github.com/flidai/leapview/internal/dashboard/runtimefactory"
)

type RuntimeFactoryConfig = dashboardanalytics.RuntimeFactoryConfig

func NewRuntimeFactory(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return dashboardanalytics.NewRuntimeBuilder(config)
}
