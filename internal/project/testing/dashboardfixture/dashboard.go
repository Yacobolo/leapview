package dashboardfixture

import (
	"fmt"

	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	dashboarddefinition "github.com/flidai/leapview/internal/dashboard/definition"
	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	workspacecompiler "github.com/flidai/leapview/internal/project/compiler"
)

// Compile crosses the same authoring-to-serving boundary as production. Test
// doubles use it so they cannot accidentally preserve the removed authoring
// dashboard runtime interface.
func Compile(report reportdef.Dashboard, model *semanticmodel.Model) dashboarddefinition.Definition {
	visualizations, err := workspacecompiler.CompileVisualizationDefinitions(&report, model)
	if err != nil {
		panic(fmt.Sprintf("compile dashboard fixture visualizations: %v", err))
	}
	definition, err := workspacecompiler.CompileDashboardDefinition(&report, visualizations)
	if err != nil {
		panic(fmt.Sprintf("compile dashboard fixture: %v", err))
	}
	return definition
}
