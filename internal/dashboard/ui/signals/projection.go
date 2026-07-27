package signals

import (
	"fmt"
	"sort"
	"strings"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	"github.com/Yacobolo/leapview/internal/dashboard"
	dashboarddefinition "github.com/Yacobolo/leapview/internal/dashboard/definition"
	visualizationdefinition "github.com/Yacobolo/leapview/internal/dashboard/visualization/definition"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
	visualizationruntime "github.com/Yacobolo/leapview/internal/dashboard/visualization/runtime"
)

const RouteDashboard RouteKind = "dashboard"
const dashboardAgentReferenceLimit int32 = 12

func DashboardInitialEnvelope(clientID, streamInstanceID string, catalog dashboard.Catalog, report dashboarddefinition.Definition, model *semanticmodel.Model, definitions map[string]visualizationdefinition.Definition, pages []dashboard.Page, activePage dashboard.Page, initialFilters dashboard.Filters) DashboardEnvelope {
	activePage = activePage.WithDefaults()
	tableRequest := DefaultTableRequest(report, activePage)
	initialFilters = report.NormalizeFiltersForPage(activePage.ID, initialFilters).WithDefaults()
	modelID, modelTitle := "", ""
	if model != nil {
		modelID = model.Name
		modelTitle = model.Title
	}
	return DashboardEnvelope{
		Agent: ChatSignal{
			Conversations: []ChatConversationSummary{},
			Transcript:    []ChatTranscriptItemSignal{},
			Status: ChatStatus{
				Enabled: false,
				Running: false,
				Error:   optionalValue("Agent is not configured"),
			},
			Composer: ComposerSignal{
				Disabled:    true,
				Placeholder: "Agent is not configured",
			},
		},
		AgentContext: AgentContextSignal{
			Surface:        "dashboard",
			WorkspaceID:    catalog.Workspace.ID,
			DashboardID:    report.ID,
			DashboardTitle: report.Title,
			PageID:         activePage.ID,
			PageTitle:      activePage.Title,
			ModelID:        modelID,
			Filters:        DashboardFiltersFromDashboard(initialFilters),
			ReferenceLimit: dashboardAgentReferenceLimit,
			References:     []AgentReferenceSignal{},
		},
		AgentReferenceSearch: AgentReferenceSearchSignal{Results: []AgentReferenceSignal{}},
		AgentVisuals:         map[string]visualizationir.VisualizationEnvelope{},
		Page: DashboardPageSignal{
			Kind:           RouteDashboard,
			Presentation:   "app",
			Title:          report.Title,
			Description:    optionalValue(report.Description),
			DashboardID:    report.ID,
			DashboardTitle: report.Title,
			PageID:         activePage.ID,
			PageTitle:      activePage.Title,
			HeaderDetail:   ReportPageHeaderDetail(pages, activePage),
			ModelID:        modelID,
			ModelTitle:     modelTitle,
			Canvas:         DashboardPageCanvasFromDashboard(activePage.Canvas),
			Grid:           DashboardPageGridFromDashboard(activePage.Grid),
			Pages:          dashboardPageNav(catalog.Workspace.ID, report.ID, pages, activePage),
			Components:     dashboardComponents(activePage),
		},
		Runtime: RouteRuntimeSignal{
			Kind:             RouteDashboard,
			ClientID:         optionalValue(clientID),
			StreamInstanceID: optionalValue(streamInstanceID),
			DashboardID:      optionalValue(report.ID),
			PageID:           optionalValue(activePage.ID),
			ModelID:          optionalValue(modelID),
		},
		FilterConfig:               ReportFilterConfigsFromReport(report.FilterConfigForPage(activePage.ID)),
		Filters:                    DashboardFiltersFromDashboard(initialFilters),
		URLParams:                  report.URLParamsFromFiltersForPage(activePage.ID, initialFilters),
		URLParamShape:              report.URLParamShapeForPage(activePage.ID),
		FilterOptions:              map[string][]DashboardFilterOption{},
		InteractionCommand:         DashboardInteractionCommandFromDashboard(dashboard.InteractionCommand{Toggle: true, Mappings: []dashboard.InteractionCommandMapping{}}),
		VisualWindowCommand:        DashboardVisualWindowRequestFromDashboard(tableRequest),
		VisualSpatialWindowCommand: DashboardVisualSpatialWindowRequestFromDashboard(dashboard.SpatialWindowRequest{}),
		Visuals:                    InitialVisualizationEnvelopes(definitions, activePage, tableRequest),
		Status:                     DashboardStatusFromDashboard(dashboard.Status{}),
	}
}

func DefaultTableRequest(report dashboarddefinition.Definition, page dashboard.Page) dashboard.TableRequest {
	request := dashboard.TableRequest{Block: "all", Count: dashboard.TableChunkSize}
	for _, name := range pageVisualIDs(page) {
		table, ok := report.Visualizations[name]
		if !ok || table.Query.Kind != visualizationdefinition.QueryDetail {
			continue
		}
		request.Table = name
		if len(table.Query.Detail.DefaultSort) > 0 {
			request.Sort = dashboard.TableSort{
				Key:       table.Query.Detail.DefaultSort[0].FieldID,
				Direction: table.Query.Detail.DefaultSort[0].Direction,
			}
		}
		break
	}
	return request
}

func InitialVisualizationEnvelopes(definitions map[string]visualizationdefinition.Definition, page dashboard.Page, request dashboard.TableRequest) map[string]DashboardVisualizationSignal {
	ids := pageVisualIDs(page)
	out := make(map[string]DashboardVisualizationSignal, len(ids))
	for _, id := range ids {
		definition, ok := definitions[id]
		if !ok {
			panic(fmt.Sprintf("compiled dashboard visualization %q is missing from initial signals", id))
		}
		dataRevision := int64(1)
		resetVersion := int64(0)
		if definition.Query.Kind == visualizationdefinition.QueryDetail || definition.Query.Kind == visualizationdefinition.QueryMatrix || definition.Query.Kind == visualizationdefinition.QueryPivot {
			resetVersion = int64(request.ResetVersion)
			dataRevision = int64(max(request.ResetVersion, 1))
		}
		envelope, err := visualizationruntime.EmptyEnvelopeFromDefinition(definition, dataRevision, 1, resetVersion)
		if err != nil {
			panic(fmt.Sprintf("compiled dashboard visualization %q has invalid initial envelope: %v", id, err))
		}
		out[id] = DashboardVisualizationSignalFromIR(envelope)
	}
	return out
}

func ReportPageHeaderDetail(pages []dashboard.Page, activePage dashboard.Page) string {
	title := displayLabel(activePage.Title, activePage.ID)
	for index, page := range pages {
		if page.ID == activePage.ID {
			return formatReportPageNumber(index, len(pages)) + ". " + title
		}
	}
	return title
}

func ValidateDashboardEnvelope(envelope DashboardEnvelope) error {
	if envelope.Page.Kind != RouteDashboard {
		return fmt.Errorf("dashboard envelope page kind = %q", envelope.Page.Kind)
	}
	if envelope.Runtime.Kind != RouteDashboard {
		return fmt.Errorf("dashboard envelope runtime kind = %q", envelope.Runtime.Kind)
	}
	if envelope.Page.DashboardID == "" || envelope.Page.PageID == "" {
		return fmt.Errorf("dashboard envelope requires dashboardId and pageId")
	}
	usedVisuals := map[string]struct{}{}
	for _, component := range envelope.Page.Components {
		switch {
		case component.Visual != nil && *component.Visual != "":
			usedVisuals[*component.Visual] = struct{}{}
			if _, ok := envelope.Visuals[*component.Visual]; !ok {
				return fmt.Errorf("component %q references missing visual %q", component.ID, *component.Visual)
			}
		case component.Filter != nil && *component.Filter != "":
			if !filterConfigContains(envelope.FilterConfig, *component.Filter) {
				return fmt.Errorf("component %q references missing filter config %q", component.ID, *component.Filter)
			}
			if _, ok := envelope.Filters.Controls[*component.Filter]; !ok {
				return fmt.Errorf("component %q references missing filter control %q", component.ID, *component.Filter)
			}
		}
	}
	for id := range envelope.Visuals {
		if _, ok := usedVisuals[id]; !ok {
			return fmt.Errorf("unused visual payload %q", id)
		}
	}
	return nil
}

func dashboardPageNav(workspaceID, reportID string, pages []dashboard.Page, activePage dashboard.Page) []DashboardPageNavSignal {
	items := make([]DashboardPageNavSignal, 0, len(pages))
	for _, page := range pages {
		items = append(items, DashboardPageNavSignal{
			ID: page.ID, Title: page.Title,
			Href:   "/workspaces/" + workspaceID + "/dashboards/" + reportID + "/pages/" + page.ID,
			Active: page.ID == activePage.ID,
		})
	}
	return items
}

func dashboardComponents(page dashboard.Page) []DashboardComponentSignal {
	components := make([]DashboardComponentSignal, 0, len(page.Visuals))
	for _, visual := range page.PlacedVisuals() {
		components = append(components, DashboardComponentSignal{
			ID: visual.ID, Kind: visual.Kind, Visual: optionalValue(visual.Visual), Filter: optionalValue(visual.Filter),
			Description: optionalValue(visual.Description), Placement: DashboardPagePlacementFromDashboard(visual.Placement),
			X: visual.X, Y: visual.Y, Width: visual.Width, Height: visual.Height,
			Eyebrow: optionalValue(visual.Eyebrow), Title: optionalValue(visual.Title),
			Subtitle: optionalValue(visual.Subtitle), Badges: optionalSlice(visual.Badges),
		})
	}
	return components
}

func pageVisualIDs(page dashboard.Page) []string {
	seen := map[string]struct{}{}
	ids := []string{}
	for _, item := range page.Visuals {
		if item.Visual == "" {
			continue
		}
		if _, ok := seen[item.Visual]; ok {
			continue
		}
		seen[item.Visual] = struct{}{}
		ids = append(ids, item.Visual)
	}
	sort.Strings(ids)
	return ids
}

func displayLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

func formatReportPageNumber(index, pageCount int) string {
	pageNumber := fmt.Sprintf("%d", index+1)
	if pageCount >= 10 {
		width := len(fmt.Sprintf("%d", pageCount))
		if len(pageNumber) < width {
			return strings.Repeat("0", width-len(pageNumber)) + pageNumber
		}
	}
	return pageNumber
}

func filterConfigContains(config []ReportFilterConfig, id string) bool {
	for _, item := range config {
		if item.ID == id {
			return true
		}
	}
	return false
}
