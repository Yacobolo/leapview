package ui

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/Yacobolo/leapview/internal/dashboard"
	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
	catalog "github.com/Yacobolo/leapview/internal/workspace/navigation"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
	"github.com/Yacobolo/leapview/pkg/pagestream"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func staticAsset(path string) string {
	return staticasset.URL(path)
}

func datastarScriptURL() string {
	return staticAsset(staticasset.DatastarScriptPath)
}

const appRootClass = "min-h-svh bg-app text-fg-default"

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

func inspectorScript() g.Node {
	if staticasset.Production() {
		return nil
	}
	return h.Script(h.Type("module"), h.Src(staticAsset("/static/datastar-inspector.js")))
}

func inspectorElement() g.Node {
	if staticasset.Production() {
		return nil
	}
	return g.El("datastar-inspector", g.Attr("signals-url", "/__dev/pagestream/signals"))
}

func pageHead(extra ...g.Node) []g.Node {
	nodes := []g.Node{
		h.Link(h.Rel("icon"), h.Href(staticAsset(defaultFaviconPath)), h.Type("image/svg+xml")),
		h.Link(h.Rel("stylesheet"), h.Href(staticAsset("/static/app.css"))),
		h.Script(h.Src(staticAsset("/static/theme.js"))),
		h.Script(h.Type("module"), h.Src(staticAsset("/static/command.js"))),
	}
	return append(nodes, extra...)
}

func csrfMeta(token string) g.Node {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return h.Meta(h.Name("csrf-token"), h.Content(token))
}

func CatalogPage(catalog catalog.Catalog, chromeOptions ...ChromeOption) g.Node {
	return catalogPageDocument(catalog, catalogPageSignal(catalog), chromeOptions...)
}

func CatalogPageForCatalogs(catalogs []catalog.Catalog, chromeOptions ...ChromeOption) g.Node {
	if len(catalogs) == 0 {
		return CatalogPage(catalog.Catalog{}, chromeOptions...)
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
	return catalogPageDocument(catalogs[0], page, chromeOptions...)
}

func catalogPageDocument(catalog catalog.Catalog, page uisignals.CatalogPageSignal, chromeOptions ...ChromeOption) g.Node {
	chrome := uisignals.ChromeSignal{Sidebar: uisignals.SidebarConfigForCatalog(catalog)}
	applyChromeOptions(&chrome, chromeOptions)
	catalogUpdatesURL := updatesURL(uisignals.RouteCatalog)
	return pagestream.RenderPage(pagestream.PageSpec{
		Title:             defaultProductName + " Dashboards",
		DatastarScriptURL: datastarScriptURL(),
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: pageHead(
			h.Script(h.Type("module"), h.Src(staticAsset("/static/app-shell.js"))),
			h.Script(h.Type("module"), h.Src(staticAsset("/static/catalog-page.js"))),
			inspectorScript(),
		),
		MainAttrs:  []g.Node{h.Class(appRootClass)},
		UpdatesURL: catalogUpdatesURL,
		Body: []g.Node{
			g.El("lv-app-shell",
				g.El("lv-catalog-page",
					g.Attr("slot", "page"),
				),
			),
			inspectorElement(),
		},
	})
}

func CatalogBootstrapSignals(catalog catalog.Catalog, chromeOptions ...ChromeOption) map[string]any {
	return CatalogBootstrapSignalsForPage(catalog, catalogPageSignal(catalog), chromeOptions...)
}

func CatalogBootstrapSignalsForCatalogs(catalogs []catalog.Catalog, chromeOptions ...ChromeOption) map[string]any {
	if len(catalogs) == 0 {
		return CatalogBootstrapSignals(catalog.Catalog{}, chromeOptions...)
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
	return CatalogBootstrapSignalsForPage(catalogs[0], page, chromeOptions...)
}

func CatalogBootstrapSignalsForPage(catalog catalog.Catalog, page uisignals.CatalogPageSignal, chromeOptions ...ChromeOption) map[string]any {
	chrome := uisignals.ChromeSignal{Sidebar: uisignals.SidebarConfigForCatalog(catalog)}
	applyChromeOptions(&chrome, chromeOptions)
	return map[string]any{
		"chrome": chrome,
		"page":   page,
		"status": dashboard.Status{},
	}
}

type recordTable = uisignals.RecordTableSignal
type recordTableColumn = uisignals.RecordTableColumnSignal
type recordTableBadge = uisignals.RecordTableBadgeSignal

type ChromeOption func(*uisignals.ChromeSignal)

func WithChatSidebar(signal ChatSignal) ChromeOption {
	return func(chrome *uisignals.ChromeSignal) {
		uisignals.AttachChatSidebar(&chrome.Sidebar, signal)
	}
}

func applyChromeOptions(chrome *uisignals.ChromeSignal, options []ChromeOption) {
	for _, option := range options {
		if option != nil {
			option(chrome)
		}
	}
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
