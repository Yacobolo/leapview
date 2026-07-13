package command

import (
	"context"
	"fmt"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/dashboard/reportmodel"
)

type Metrics interface {
	report.Metrics
	NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest
	QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error)
	RefreshMaterializations(ctx context.Context, modelID string) error
	DataDir() string
}

type Service struct {
	Metrics Metrics
}

type Request struct {
	DashboardID        string
	PageID             string
	ModelID            string
	Filters            dashboard.Filters
	TableCommand       dashboard.TableRequest
	InteractionCommand dashboard.InteractionCommand
}

type EventType string

const (
	EventLoading   EventType = "loading"
	EventDashboard EventType = "dashboard"
	EventTables    EventType = "tables"
	EventTable     EventType = "table"
)

type Event struct {
	Type      EventType
	DataDir   string
	Patch     dashboard.Patch
	Tables    map[string]dashboard.Table
	TableName string
	Table     dashboard.Table
}

func (s Service) TableWindow(ctx context.Context, request Request) []Event {
	tableRequest := s.Metrics.NormalizeTableRequest(request.DashboardID, request.TableCommand)
	filters := report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, request.Filters)
	table := report.QueryTable(ctx, s.Metrics, request.DashboardID, request.PageID, filters, tableRequest)
	if report.IsCanceledTable(table) {
		return nil
	}
	return []Event{{
		Type:      EventTable,
		TableName: tableRequest.Table,
		Table:     table,
	}}
}

func (s Service) Select(ctx context.Context, request Request) []Event {
	filters := report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, request.Filters)
	command, err := canonicalInteractionCommand(s.Metrics, request.DashboardID, request.InteractionCommand)
	if err != nil {
		return []Event{
			{Type: EventLoading, DataDir: s.Metrics.DataDir()},
			{Type: EventDashboard, Patch: dashboard.EmptyPatch(filters, s.Metrics.DataDir(), fmt.Errorf("invalid interaction selection: %w", err))},
		}
	}
	filters = filters.ApplyInteraction(command)
	filters = report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, filters)
	return s.reload(ctx, request, filters)
}

func canonicalInteractionCommand(metrics Metrics, dashboardID string, command dashboard.InteractionCommand) (dashboard.InteractionCommand, error) {
	definition, model, ok := metrics.Report(dashboardID)
	if !ok || model == nil {
		return dashboard.InteractionCommand{}, fmt.Errorf("dashboard %q is not published", dashboardID)
	}
	wantKind := "point_selection"
	var toggle bool
	semanticMappingCount := 0
	switch command.SourceKind {
	case "visual":
		source, ok := definition.Visuals[command.SourceID]
		if !ok {
			return dashboard.InteractionCommand{}, fmt.Errorf("unknown source visual %q", command.SourceID)
		}
		toggle = source.Interaction.PointSelection.Toggle
		semanticMappingCount = len(source.Interaction.PointSelection.Mappings)
	case "table":
		wantKind = "row_selection"
		source, ok := definition.Tables[command.SourceID]
		if !ok {
			return dashboard.InteractionCommand{}, fmt.Errorf("unknown source table %q", command.SourceID)
		}
		toggle = source.Interaction.RowSelection.Toggle
		semanticMappingCount = len(source.Interaction.RowSelection.Mappings)
	default:
		return dashboard.InteractionCommand{}, fmt.Errorf("unknown source kind %q", command.SourceKind)
	}
	if command.InteractionKind != wantKind {
		return dashboard.InteractionCommand{}, fmt.Errorf("source %s %q requires interaction kind %q", command.SourceKind, command.SourceID, wantKind)
	}
	if command.Action != "set" && command.Action != "replace" && command.Action != "clear" {
		return dashboard.InteractionCommand{}, fmt.Errorf("unsupported selection action %q", command.Action)
	}
	command.Toggle = toggle
	if command.Action == "clear" {
		command.Mappings = nil
		return command, nil
	}
	if command.SourceKind == "table" && semanticMappingCount == 0 {
		if len(command.Mappings) != 1 || command.Mappings[0].Field != dashboard.UIRowSelectionField || command.Mappings[0].Fact != "" || command.Mappings[0].Grain != "" || !dashboard.IsInteractionSelectionScalar(command.Mappings[0].Value) {
			return dashboard.InteractionCommand{}, fmt.Errorf("table %q without semantic selection mappings accepts only the UI row key", command.SourceID)
		}
		return command, nil
	}
	if semanticMappingCount == 0 {
		return dashboard.InteractionCommand{}, fmt.Errorf("%s %q has no semantic selection mappings", command.SourceKind, command.SourceID)
	}
	resolved, err := reportmodel.ResolveSelectionInteraction(&definition, model, command.SourceKind, command.SourceID)
	if err != nil {
		return dashboard.InteractionCommand{}, err
	}
	identities := make([]reportmodel.SelectionMappingIdentity, len(command.Mappings))
	incoming := make(map[reportmodel.SelectionMappingIdentity]dashboard.InteractionCommandMapping, len(command.Mappings))
	for index, mapping := range command.Mappings {
		if !mapping.HasValue() {
			return dashboard.InteractionCommand{}, fmt.Errorf("mapping %d must include value", index)
		}
		if !dashboard.IsInteractionSelectionScalar(mapping.Value) {
			return dashboard.InteractionCommand{}, fmt.Errorf("mapping %d value must be a JSON scalar", index)
		}
		identity := reportmodel.SelectionMappingIdentity{Field: mapping.Field, Fact: mapping.Fact, Grain: mapping.Grain}
		identities[index] = identity
		incoming[identity] = mapping
	}
	canonical, err := resolved.CanonicalizeMappings(identities)
	if err != nil {
		return dashboard.InteractionCommand{}, err
	}
	command.Mappings = make([]dashboard.InteractionCommandMapping, 0, len(canonical))
	for _, mapping := range canonical {
		identity := reportmodel.SelectionMappingIdentity{Field: mapping.Field, Fact: mapping.Fact, Grain: mapping.Grain}
		value := incoming[identity]
		if !dashboard.InteractionSelectionValueMatchesType(value.Value, mapping.Type, mapping.Grain) {
			return dashboard.InteractionCommand{}, fmt.Errorf("mapping field %q value type %T does not match semantic type %q", mapping.Field, value.Value, mapping.Type)
		}
		command.Mappings = append(command.Mappings, value)
	}
	return command, nil
}

func (s Service) ClearSelection(ctx context.Context, request Request) []Event {
	filters := report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, request.Filters)
	filters.Selections = nil
	return s.reload(ctx, request, filters)
}

func (s Service) Reload(ctx context.Context, request Request) []Event {
	filters := report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, request.Filters)
	return s.reload(ctx, request, filters)
}

func (s Service) ResetFilters(ctx context.Context, request Request) []Event {
	return s.reload(ctx, request, report.DefaultFilters(s.Metrics, request.DashboardID, request.PageID))
}

func (s Service) RefreshMaterializations(ctx context.Context, request Request) []Event {
	filters := report.NormalizeFilters(s.Metrics, request.DashboardID, request.PageID, request.Filters)
	events := []Event{{Type: EventLoading, DataDir: s.Metrics.DataDir()}}
	if err := s.Metrics.RefreshMaterializations(ctx, request.ModelID); err != nil {
		events = append(events, Event{
			Type:  EventDashboard,
			Patch: dashboard.EmptyPatch(filters, s.Metrics.DataDir(), err),
		})
		return events
	}
	return append(events, s.reloadEvents(ctx, request, filters)...)
}

func (s Service) reload(ctx context.Context, request Request, filters dashboard.Filters) []Event {
	events := []Event{{Type: EventLoading, DataDir: s.Metrics.DataDir()}}
	return append(events, s.reloadEvents(ctx, request, filters)...)
}

func (s Service) reloadEvents(ctx context.Context, request Request, filters dashboard.Filters) []Event {
	tableRequest := s.Metrics.NormalizeTableRequest(request.DashboardID, request.TableCommand).Reset()
	patch, err := s.Metrics.QueryDashboardPage(ctx, request.DashboardID, request.PageID, filters)
	if err != nil {
		patch = dashboard.EmptyPatch(filters, s.Metrics.DataDir(), err)
	}
	return []Event{
		{Type: EventDashboard, Patch: patch},
		{Type: EventTables, Tables: report.Tables(ctx, s.Metrics, request.DashboardID, request.PageID, filters, tableRequest)},
	}
}
