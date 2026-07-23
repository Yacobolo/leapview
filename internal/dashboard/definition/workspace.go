package definition

import (
	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	"github.com/Yacobolo/leapview/internal/catalog"
)

// Workspace is the immutable dashboard capability projection of a compiled
// project workspace. Individual dashboards remain immutable Definitions.
type Workspace struct {
	Catalog    catalog.Catalog
	Models     map[string]*semanticmodel.Model
	Dashboards map[string]Definition
}
