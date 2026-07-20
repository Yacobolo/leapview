package app

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/agent"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
)

func TestResolveDashboardTurnReferencesUsesCompiledMetadata(t *testing.T) {
	page := dashboard.Page{Visuals: []dashboard.PageVisual{
		{ID: "orders-chart", Visual: "orders_chart"},
		{ID: "orders-table", Table: "orders", Title: "Recent orders"},
	}}
	resolved := resolveDashboardTurnReferences([]agent.TurnReference{
		{Kind: "visual", ComponentID: "orders-chart", VisualID: "orders_chart", Title: "Ignore browser title", VisualType: "script"},
		{Kind: "visual", ComponentID: "orders-table", VisualID: "orders", Title: "Ignore browser table title"},
		{Kind: "visual", ComponentID: "off-page", VisualID: "secret", Title: "Not on page"},
	}, page, map[string]reportdef.Visual{
		"orders_chart": {Title: "Orders by status", Type: "bar"},
		"secret":       {Title: "Secret", Type: "line"},
	}, map[string]reportdef.TableVisual{
		"orders": {Title: "Orders", Kind: "table"},
	})

	want := []agent.TurnReference{
		{Kind: "visual", ComponentID: "orders-chart", VisualID: "orders_chart", Title: "Orders by status", VisualType: "bar"},
		{Kind: "visual", ComponentID: "orders-table", VisualID: "orders", Title: "Recent orders", VisualType: "table"},
	}
	if len(resolved) != len(want) {
		t.Fatalf("resolved references = %#v, want %#v", resolved, want)
	}
	for index := range want {
		if resolved[index] != want[index] {
			t.Fatalf("resolved[%d] = %#v, want %#v", index, resolved[index], want[index])
		}
	}
}
