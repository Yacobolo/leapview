package report

import (
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/dashboard"
)

func TestDashboardRejectsUnknownTableCardinalityPolicy(t *testing.T) {
	dashboardDefinition := Dashboard{
		ID:            "commerce",
		Title:         "Commerce",
		SemanticModel: "commerce",
		Visuals: map[string]AuthoringVisualization{
			"orders": TabularVisualization("table", TableVisual{Title: "Orders", Cardinality: "automatic", Query: TableQuery{Table: "orders", Fields: []string{"order_id"}}}),
		},
		Pages: []dashboard.Page{{ID: "overview", Title: "Overview", Visuals: []dashboard.PageVisual{{Kind: "table", Visual: "orders"}}}},
	}
	err := dashboardDefinition.ValidateContract()
	if err == nil || !strings.Contains(err.Error(), "unsupported cardinality") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestTableCardinalityDefaultsToBounded(t *testing.T) {
	if got := (TableVisual{}).CardinalityOrDefault(); got != TableCardinalityBounded {
		t.Fatalf("default cardinality = %q", got)
	}
}
