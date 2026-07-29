package module

import (
	"testing"

	"github.com/flidai/leapview/internal/dashboard"
	dashboarddefinition "github.com/flidai/leapview/internal/dashboard/definition"
	dashboardfilter "github.com/flidai/leapview/internal/dashboard/filter"
)

func TestCatalogFilterComponentResolvesTypedFilterBindings(t *testing.T) {
	report := dashboarddefinition.Definition{
		FilterDefinitions: map[string]dashboardfilter.Definition{
			"state": {Label: "State", Field: "orders.state"},
		},
		FilterBindings: map[string]dashboardfilter.Binding{
			"report_state": {ID: "report_state", Filter: "state", Scope: dashboardfilter.ScopeReport},
		},
	}
	page := dashboard.Page{
		ID: "overview",
		FilterBindings: map[string]dashboardfilter.Binding{
			"page_state": {ID: "page_state", Filter: "state", Scope: dashboardfilter.ScopePage, PageID: "overview"},
		},
		Visuals: []dashboard.PageVisual{
			{ID: "report-slicer", Binding: dashboardfilter.BindingRef{Scope: dashboardfilter.ScopeReport, ID: "report_state"}},
			{ID: "page-slicer", Binding: dashboardfilter.BindingRef{Scope: dashboardfilter.ScopePage, ID: "page_state"}},
		},
	}

	component, binding, ok := catalogFilterComponent(report, page, "state")
	if !ok {
		t.Fatal("typed filter binding was not resolved")
	}
	if component.ID != "report-slicer" || binding.Filter != "state" {
		t.Fatalf("resolved component = %#v, binding = %#v", component, binding)
	}
	projected := catalogComponent(component, report, page)
	if projected["kind"] != "filter" || projected["ref"] != "state" || projected["title"] != "State" {
		t.Fatalf("catalog component = %#v", projected)
	}
}
