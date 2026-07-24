package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yacobolo/leapview/internal/dashboard"
	visualizationir "github.com/Yacobolo/leapview/internal/visualization/ir"
)

func TestDashboardVisualAgentProjectionUsesCanonicalSpecKind(t *testing.T) {
	base := visualizationir.VisualizationSpecBase{
		Title: "Revenue",
		Datasets: []visualizationir.VisualizationDatasetSchema{{
			ID: "primary",
			Fields: []visualizationir.VisualizationField{{
				ID: "value", Label: "Revenue", Role: visualizationir.VisualizationFieldRoleMeasure,
				DataType: visualizationir.VisualizationDataTypeDecimal,
			}},
		}},
	}
	envelope := visualizationir.VisualizationEnvelope{
		VisualID: "revenue_kpi",
		Spec: visualizationir.VisualizationSpec{Value: &visualizationir.KPIVisualizationSpec{
			VisualizationSpecBase: base,
			Kind:                  "kpi",
			Value:                 visualizationir.VisualizationFieldRef{Dataset: "primary", Field: "value"},
		}},
		DataState: visualizationir.VisualizationDataState{Value: &visualizationir.InlineVisualizationDataState{
			Kind: "inline",
			Datasets: []visualizationir.VisualizationInlineDataset{{
				ID: "primary", Columns: []string{"value"}, Rows: [][]any{{16008872.12}},
			}},
		}},
	}
	request := httptest.NewRequest(nethttp.MethodGet, "/workspaces/workspace/dashboards/dash/visuals/revenue_kpi/query", nil)

	result, err := (Handler{}).dashboardVisualAgentProjection(
		request, fakeMetrics{}, envelope, dashboard.Filters{}, 0, maxAgentDashboardVisualRows, "scope", "snapshot",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "kpi" {
		t.Fatalf("type = %q, want kpi", result.Type)
	}
}
