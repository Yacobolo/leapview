package ui

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/Yacobolo/leapview/internal/dashboard"
	webpage "github.com/Yacobolo/leapview/internal/platform/web/page"
	catalog "github.com/Yacobolo/leapview/internal/workspace/navigation"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
	g "maragu.dev/gomponents"
)

func updatesURL(route uisignals.RouteKind, pairs ...string) string {
	values := url.Values{}
	values.Set("route", string(route))
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			continue
		}
		values.Set(pairs[i], pairs[i+1])
	}
	return "/updates?" + values.Encode()
}

func runtimeSignal(kind uisignals.RouteKind) uisignals.RouteRuntimeSignal {
	return uisignals.RouteRuntimeSignal{
		Kind: kind,
	}
}

func CatalogPage(catalog catalog.Catalog, providers ...webpage.Provider) g.Node {
	return catalogPageDocument(catalog, catalogPageSignal(catalog), providers...)
}

func CatalogPageForCatalogs(catalogs []catalog.Catalog, providers ...webpage.Provider) g.Node {
	if len(catalogs) == 0 {
		return CatalogPage(catalog.Catalog{}, providers...)
	}
	dashboards := []uisignals.CatalogDashboardSignal{}
	for _, catalog := range catalogs {
		for _, report := range catalog.Dashboards {
			dashboards = append(dashboards, uisignals.CatalogDashboardSignal{
				ID:            catalog.Workspace.ID + "." + report.ID,
				Title:         report.Title,
				Description:   uisignals.Optional(report.Description),
				SemanticModel: uisignals.Optional(report.SemanticModel),
				PageCount:     int64(report.PageCount),
				Tags:          uisignals.OptionalSlice(report.Tags),
				Href:          "/workspaces/" + catalog.Workspace.ID + "/dashboards/" + report.ID,
			})
		}
	}
	page := catalogPageSignal(catalogs[0])
	page.Dashboards = dashboards
	return catalogPageDocument(catalogs[0], page, providers...)
}

func catalogPageDocument(catalog catalog.Catalog, page uisignals.CatalogPageSignal, providers ...webpage.Provider) g.Node {
	layout := webpage.Resolve(firstProvider(providers), catalogLayoutContext(catalog))
	catalogUpdatesURL := updatesURL(uisignals.RouteCatalog)
	title := "Dashboards"
	if productName := strings.TrimSpace(layout.Presentation.ProductName); productName != "" {
		title = productName + " Dashboards"
	}
	return webpage.Render(layout, webpage.Spec{
		Title: title, Scripts: []string{"/static/catalog-page.js"},
		UpdatesURL: catalogUpdatesURL,
		Content:    g.El("lv-catalog-page", g.Attr("slot", "page")),
	})
}

func CatalogBootstrapSignals(catalog catalog.Catalog, providers ...webpage.Provider) map[string]any {
	return CatalogBootstrapSignalsForPage(catalog, catalogPageSignal(catalog), providers...)
}

func CatalogBootstrapSignalsForCatalogs(catalogs []catalog.Catalog, providers ...webpage.Provider) map[string]any {
	if len(catalogs) == 0 {
		return CatalogBootstrapSignals(catalog.Catalog{}, providers...)
	}
	dashboards := []uisignals.CatalogDashboardSignal{}
	for _, catalog := range catalogs {
		for _, report := range catalog.Dashboards {
			dashboards = append(dashboards, uisignals.CatalogDashboardSignal{
				ID:            catalog.Workspace.ID + "." + report.ID,
				Title:         report.Title,
				Description:   uisignals.Optional(report.Description),
				SemanticModel: uisignals.Optional(report.SemanticModel),
				PageCount:     int64(report.PageCount),
				Tags:          uisignals.OptionalSlice(report.Tags),
				Href:          "/workspaces/" + catalog.Workspace.ID + "/dashboards/" + report.ID,
			})
		}
	}
	page := catalogPageSignal(catalogs[0])
	page.Dashboards = dashboards
	return CatalogBootstrapSignalsForPage(catalogs[0], page, providers...)
}

func CatalogBootstrapSignalsForPage(catalog catalog.Catalog, page uisignals.CatalogPageSignal, providers ...webpage.Provider) map[string]any {
	layout := webpage.Resolve(firstProvider(providers), catalogLayoutContext(catalog))
	return webpage.WithSignal(layout, map[string]any{
		"page":   page,
		"status": dashboard.Status{},
	})
}

type recordTable = uisignals.RecordTableSignal
type recordTableColumn = uisignals.RecordTableColumnSignal
type recordTableBadge struct {
	Label string  `json:"label"`
	Tone  *string `json:"tone,omitempty"`
}

func catalogLayoutContext(catalog catalog.Catalog) webpage.Context {
	context := webpage.Context{
		Active:       "dashboards",
		SectionTitle: "Dashboards", PageTitle: "Discovery",
	}
	if len(catalog.Models) > 0 {
		context.RelatedID = catalog.Models[0].ID
		context.RelatedTitle = catalog.Models[0].Title
	}
	return context
}

func workspaceLayoutContext(catalog catalog.Catalog, active string) webpage.Context {
	return webpage.Context{
		Active: active, ScopeID: catalog.Workspace.ID, ScopeTitle: catalog.Workspace.Title,
		SectionTitle: "Workspace", PageTitle: "Published assets",
	}
}

func firstProvider(providers []webpage.Provider) webpage.Provider {
	if len(providers) == 0 {
		return nil
	}
	return providers[0]
}

func catalogPageSignal(catalog catalog.Catalog) uisignals.CatalogPageSignal {
	dashboards := make([]uisignals.CatalogDashboardSignal, 0, len(catalog.Dashboards))
	for _, report := range catalog.Dashboards {
		dashboards = append(dashboards, uisignals.CatalogDashboardSignal{
			ID:            report.ID,
			Title:         report.Title,
			Description:   uisignals.Optional(report.Description),
			SemanticModel: uisignals.Optional(report.SemanticModel),
			PageCount:     int64(report.PageCount),
			Tags:          uisignals.OptionalSlice(report.Tags),
			Href:          "/workspaces/" + catalog.Workspace.ID + "/dashboards/" + report.ID,
		})
	}
	return uisignals.CatalogPageSignal{
		Kind:        uisignals.RouteCatalog,
		Title:       "Dashboards",
		Description: "Reports backed by semantic models.",
		Dashboards:  dashboards,
	}
}

func recordTableBadgeValue(value, tone string) any {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return recordTableBadge{Label: value, Tone: uisignals.Optional(tone)}
}

func displayLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func jsonString(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
