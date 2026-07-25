package ui

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	dashboarddefinition "github.com/Yacobolo/leapview/internal/dashboard/definition"
	uisignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
	visualizationdefinition "github.com/Yacobolo/leapview/internal/dashboard/visualization/definition"
	uiactions "github.com/Yacobolo/leapview/internal/platform/web/actions"
	webpage "github.com/Yacobolo/leapview/internal/platform/web/page"

	"github.com/Yacobolo/leapview/internal/dashboard"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func updatesURL(workspaceID, dashboardID, pageID string) string {
	values := url.Values{}
	values.Set("route", string(uisignals.RouteDashboard))
	values.Set("workspace", workspaceID)
	values.Set("dashboard", dashboardID)
	values.Set("page", pageID)
	return "/updates?" + values.Encode()
}

func updatesURLWithParams(workspaceID, dashboardID, pageID string, params map[string]any) string {
	values := url.Values{}
	values.Set("route", string(uisignals.RouteDashboard))
	values.Set("workspace", workspaceID)
	values.Set("dashboard", dashboardID)
	values.Set("page", pageID)
	for key, raw := range params {
		switch typed := raw.(type) {
		case []string:
			for _, value := range typed {
				if strings.TrimSpace(value) != "" {
					values.Add(key, value)
				}
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				values.Set(key, typed)
			}
		}
	}
	return "/updates?" + values.Encode()
}

const (
	PresentationApp    = "app"
	PresentationPublic = "public"
	PresentationEmbed  = "embed"
)

type PublicPageOptions struct {
	PublicID     string
	ClientID     string
	Presentation string
}

type Presentation = webpage.Presentation

func Page(clientID, csrfToken string, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters, providers ...webpage.Provider) g.Node {
	return PageWithPresentation(Presentation{ProductName: "Application", FaviconPath: "/static/favicon.svg"}, clientID, csrfToken, catalog, report, model, pages, activePage, initialFilters, providers...)
}

func PageWithPresentation(presentation Presentation, clientID, csrfToken string, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters, providers ...webpage.Provider) g.Node {
	if activePage.ID == "" {
		activePage = defaultPage()
	}
	visualReset := visualResetExpression()
	initialFilters = report.NormalizeFiltersForPage(activePage.ID, initialFilters)
	initialURLParams := report.URLParamsFromFiltersForPage(activePage.ID, initialFilters)
	initialURLParams["streamInstance"] = newStreamInstanceID()
	dashboardUpdatesURL := updatesURLWithParams(catalog.Workspace.ID, report.ID, activePage.ID, initialURLParams)
	reloadAction := uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/reload", "runtime", "filters.controls")
	filtersUpdate := "$filters = evt.detail.filters; $urlParams = evt.detail.urlParams; window.DatastarURLSync && window.DatastarURLSync.replace($urlParams); " + visualReset
	agentTurn := "$agent.composer.value = evt.detail.input; $agentContext.references = evt.detail.references; $agentContext.filters = $filters; $agentContext.generation = $status.generation; " + uiactions.Post("/chats/turns", "agent", "agentContext")
	agentRestore := "$agent.activeConversationId = evt.detail.conversationId; " + uiactions.Get("/chats/restore", "agent")
	provider := firstProvider(providers)
	layout := webpage.Resolve(provider, dashboardLayoutContext(catalog, report, model, activePage))
	if provider == nil {
		layout.Presentation = presentation
	}
	return webpage.Render(layout, webpage.Spec{
		Title: layout.Presentation.ProductName, CSRFToken: csrfToken,
		Scripts: []string{"/static/dashboard-page.js", "/static/url-sync.js"},
		MainAttrs: []g.Node{
			h.ID("dashboard"),
			h.Class(webpage.RootClass),
			g.Attr("data-on:datastar-url-params-sync__window", "$urlParams = evt.detail.params; $filters = window.LeapViewFilterURL.fromParams($filterConfig, $filters, $urlParams); "+visualReset+reloadAction),
		},
		UpdatesURL: dashboardUpdatesURL,
		ContentAttrs: []g.Node{
			g.Attr("data-on:lv-chat-reference-search__debounce.200ms", "$agentReferenceSearch.query = evt.detail.query; $agentReferenceSearch.requestId = evt.detail.requestId; "+uiactions.Get("/chats/references/search", "agentReferenceSearch", "agentContext")),
		},
		Content: g.El("lv-dashboard-page",
			g.Attr("slot", "page"),
			g.Attr("workspace-id", catalog.Workspace.ID),
			g.Attr("dashboard-id", report.ID),
			g.Attr("page-id", activePage.ID),
			g.Attr("data-indicator", "agentTurnPending"),
			g.Attr("data-on:lv-chat-submit", agentTurn),
			g.Attr("data-on:lv-chat-restore", agentRestore),
			g.Attr("data-on:lv-chat-new", "$agent.activeConversationId = ''; $agent.transcript = []; $agent.composer.value = ''; $agentVisuals = {}"),
			g.Attr("data-on:lv-filters-change", filtersUpdate+reloadAction),
			g.Attr("data-on:lv-filters-reset", filtersUpdate+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/reset-filters", "runtime")),
			g.Attr("data-on:lv-filters-refresh", reloadAction),
			g.Attr("data-on:lv-selection-clear", "$filters.selections = []; "+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/clear-selection", "runtime")),
			g.Attr("data-on:lv-interaction-select", "$interactionCommand = evt.detail; "+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/select", "runtime", "interactionCommand")),
			g.Attr("data-on:lv-interaction-spatial-select", "$spatialInteractionCommand = evt.detail; "+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/spatial-select", "runtime", "spatialInteractionCommand")),
			g.Attr("data-on:lv-visualization-window-request", "$visualWindowCommand = evt.detail; "+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/visual-window", "runtime", "visualWindowCommand")),
			g.Attr("data-on:lv-visual-spatial-window-change", "$visualSpatialWindowCommand = evt.detail; "+uiactions.Post("/workspaces/"+catalog.Workspace.ID+"/commands/visual-spatial-window", "runtime", "visualSpatialWindowCommand")),
		),
	})
}

// PublicPage renders the report component without authenticated application
// chrome or cookies. Every command is scoped beneath the opaque publication
// route and carries the document-generated client and stream identities.
func PublicPage(options PublicPageOptions, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters) g.Node {
	presentation := options.Presentation
	if presentation != PresentationEmbed {
		presentation = PresentationPublic
	}
	if options.ClientID == "" {
		options.ClientID = newStreamInstanceID()
	}
	initialFilters = report.NormalizeFiltersForPage(activePage.ID, initialFilters)
	params := report.URLParamsFromFiltersForPage(activePage.ID, initialFilters)
	params["streamInstance"] = newStreamInstanceID()
	params["clientId"] = options.ClientID
	values := url.Values{}
	values.Set("page", activePage.ID)
	values.Set("presentation", presentation)
	for key, raw := range params {
		switch typed := raw.(type) {
		case []string:
			for _, value := range typed {
				values.Add(key, value)
			}
		case string:
			values.Set(key, typed)
		}
	}
	base := "/public/dashboards/" + options.PublicID
	commandBase := base + "/commands/"
	visualReset := visualResetExpression()
	reloadAction := uiactions.Post(commandBase+"reload", "runtime", "filters.controls")
	filtersUpdate := "$filters = evt.detail.filters; $urlParams = evt.detail.urlParams; window.DatastarURLSync && window.DatastarURLSync.replace($urlParams); " + visualReset
	return webpage.Render(webpage.Layout{}, webpage.Spec{
		Title: report.Title, Scripts: []string{"/static/dashboard-page.js", "/static/url-sync.js"},
		MainAttrs: []g.Node{
			h.ID("dashboard"), h.Class(webpage.RootClass),
			g.Attr("data-on:datastar-url-params-sync__window", "$urlParams = evt.detail.params; $filters = window.LeapViewFilterURL.fromParams($filterConfig, $filters, $urlParams); "+visualReset+reloadAction),
		},
		UpdatesURL: base + "/updates?" + values.Encode(),
		Content: g.El("lv-dashboard-page",
			g.Attr("workspace-id", catalog.Workspace.ID), g.Attr("dashboard-id", report.ID), g.Attr("page-id", activePage.ID), g.Attr("presentation", presentation),
			g.Attr("data-on:lv-filters-change", filtersUpdate+reloadAction),
			g.Attr("data-on:lv-filters-reset", filtersUpdate+uiactions.Post(commandBase+"reset-filters", "runtime")),
			g.Attr("data-on:lv-filters-refresh", reloadAction),
			g.Attr("data-on:lv-selection-clear", "$filters.selections = []; "+uiactions.Post(commandBase+"clear-selection", "runtime")),
			g.Attr("data-on:lv-interaction-select", "$interactionCommand = evt.detail; "+uiactions.Post(commandBase+"select", "runtime", "interactionCommand")),
			g.Attr("data-on:lv-interaction-spatial-select", "$spatialInteractionCommand = evt.detail; "+uiactions.Post(commandBase+"spatial-select", "runtime", "spatialInteractionCommand")),
			g.Attr("data-on:lv-visualization-window-request", "$visualWindowCommand = evt.detail; "+uiactions.Post(commandBase+"visual-window", "runtime", "visualWindowCommand")),
			g.Attr("data-on:lv-visual-spatial-window-change", "$visualSpatialWindowCommand = evt.detail; "+uiactions.Post(commandBase+"visual-spatial-window", "runtime", "visualSpatialWindowCommand")),
		),
	})
}

func defaultPage() dashboard.Page {
	return dashboard.Page{
		ID:     "overview",
		Title:  "Overview",
		Canvas: dashboard.PageCanvas{Width: 1366, Height: 940},
		Grid:   dashboard.PageGrid{Columns: 12, RowHeight: 48, Gap: 16, Padding: 16},
	}
}

func BootstrapSignals(clientID, streamInstanceID string, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, definitions map[string]visualizationdefinition.Definition, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters, providers ...webpage.Provider) map[string]any {
	envelope := uisignals.DashboardInitialEnvelope(clientID, streamInstanceID, catalog, report, model, definitions, pages, activePage, initialFilters)
	envelope.Runtime.WorkspaceID = uisignals.Optional(catalog.Workspace.ID)
	signals := map[string]any{
		"agent":                      envelope.Agent,
		"agentContext":               envelope.AgentContext,
		"agentReferenceSearch":       envelope.AgentReferenceSearch,
		"agentVisuals":               envelope.AgentVisuals,
		"page":                       envelope.Page,
		"runtime":                    envelope.Runtime,
		"filterConfig":               envelope.FilterConfig,
		"filters":                    envelope.Filters,
		"urlParams":                  envelope.URLParams,
		"urlParamShape":              envelope.URLParamShape,
		"filterOptions":              envelope.FilterOptions,
		"interactionCommand":         envelope.InteractionCommand,
		"spatialInteractionCommand":  envelope.SpatialInteractionCommand,
		"visualWindowCommand":        envelope.VisualWindowCommand,
		"visualSpatialWindowCommand": envelope.VisualSpatialWindowCommand,
		"visuals":                    envelope.Visuals,
		"status":                     envelope.Status,
	}
	layout := webpage.Resolve(firstProvider(providers), dashboardLayoutContext(catalog, report, model, activePage))
	return webpage.WithSignal(layout, signals)
}

func PublicBootstrapSignals(clientID, streamInstanceID, publicID, presentation string, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, definitions map[string]visualizationdefinition.Definition, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters) map[string]any {
	envelope := uisignals.DashboardInitialEnvelope(clientID, streamInstanceID, catalog, report, model, definitions, pages, activePage, initialFilters)
	family := "public"
	if presentation == PresentationEmbed {
		family = "embed"
	}
	for index := range envelope.Page.Pages {
		envelope.Page.Pages[index].Href = "/" + family + "/dashboards/" + publicID + "/pages/" + envelope.Page.Pages[index].ID
	}
	envelope.Page.Presentation = presentation
	return map[string]any{
		"page":                       envelope.Page,
		"runtime":                    envelope.Runtime,
		"filterConfig":               envelope.FilterConfig,
		"filters":                    envelope.Filters,
		"urlParams":                  envelope.URLParams,
		"urlParamShape":              envelope.URLParamShape,
		"filterOptions":              envelope.FilterOptions,
		"interactionCommand":         envelope.InteractionCommand,
		"spatialInteractionCommand":  envelope.SpatialInteractionCommand,
		"visualWindowCommand":        envelope.VisualWindowCommand,
		"visualSpatialWindowCommand": envelope.VisualSpatialWindowCommand,
		"visuals":                    envelope.Visuals,
		"status":                     envelope.Status,
	}
}

func dashboardLayoutContext(catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, activePage dashboard.Page) webpage.Context {
	context := webpage.Context{
		Active: "workspaces", ScopeID: catalog.Workspace.ID, ScopeTitle: catalog.Workspace.Title,
		SectionID: report.ID, SectionTitle: report.Title,
		PageID: activePage.ID, PageTitle: activePage.Title, Compact: true,
	}
	if model != nil {
		context.RelatedID = model.Name
		context.RelatedTitle = model.Title
	}
	return context
}

func firstProvider(providers []webpage.Provider) webpage.Provider {
	if len(providers) == 0 {
		return nil
	}
	return providers[0]
}

func newStreamInstanceID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}

func visualResetExpression() string {
	count := strconv.Itoa(dashboard.TableChunkSize)
	return "$visualWindowCommand.blockID = 'all'; $visualWindowCommand.start = 0; $visualWindowCommand.limit = " + count + "; $visualWindowCommand.resetVersion = ($visualWindowCommand.resetVersion || 0) + 1; "
}
