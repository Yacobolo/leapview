package compiler

import (
	"fmt"
	"sort"
	"strings"

	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

// compileBuiltInVisualizationSpec is the canonical authoring-to-IR boundary
// for first-party charts. Runtime data never participates in specification
// construction; it is shaped later against the immutable dataset schema.
func compileBuiltInVisualizationSpec(id string, authored reportdef.Visual, model *semanticmodel.Model) (visualizationir.VisualizationSpec, error) {
	shape := authored.ResultShape()
	columns := compiledShapeColumns(shape)
	if shape == "hierarchy" {
		seen := map[string]struct{}{"node": {}, "parent": {}, "value": {}}
		for _, binding := range compiledFields(authored.Query.Dimensions) {
			if _, exists := seen[binding.Alias]; exists {
				return visualizationir.VisualizationSpec{}, fmt.Errorf("hierarchy query alias %q conflicts with a reserved frame field", binding.Alias)
			}
			seen[binding.Alias] = struct{}{}
			columns = append(columns, binding.Alias)
		}
		if binding := compiledTime(authored.Query.Time); binding != nil {
			if _, exists := seen[binding.Alias]; exists {
				return visualizationir.VisualizationSpec{}, fmt.Errorf("hierarchy query alias %q conflicts with a reserved frame field", binding.Alias)
			}
			columns = append(columns, binding.Alias)
		}
	}
	fields := make([]visualizationir.VisualizationField, len(columns))
	identities := map[string]struct{}{}
	for _, mapping := range authored.Interaction.PointSelection.Mappings {
		identities[mapping.Value] = struct{}{}
	}
	for index, column := range columns {
		role := visualizationir.VisualizationFieldRoleDimension
		if compiledShapeMeasure(column) {
			role = visualizationir.VisualizationFieldRoleMeasure
		}
		if _, ok := identities[column]; ok {
			role = visualizationir.VisualizationFieldRoleIdentity
		}
		fields[index] = visualizationir.VisualizationField{ID: column, Role: role, DataType: compiledShapeDataType(column), Nullable: true, Label: compiledShapeLabel(column)}
	}
	applyBuiltInFieldSemantics(fields, shape, authored, model)
	title := compiledVisualTitle(authored, id, model)
	accessibilityTitle := title
	if authored.Accessibility.Title != "" {
		accessibilityTitle = authored.Accessibility.Title
	}
	accessibilityDescription := title
	if authored.Accessibility.Description != "" {
		accessibilityDescription = authored.Accessibility.Description
	}
	completeness := visualizationir.VisualizationCompletenessComplete
	if authored.DataBudget.RequiredCompleteness != "" {
		completeness = visualizationir.VisualizationCompleteness(authored.DataBudget.RequiredCompleteness)
	}
	accessibility := visualizationir.VisualizationAccessibility{Title: accessibilityTitle, Description: accessibilityDescription}
	if authored.Accessibility.Summary != "" {
		accessibility.Summary = &authored.Accessibility.Summary
	}
	if authored.Accessibility.AnnounceChanges {
		accessibility.AnnounceChanges = &authored.Accessibility.AnnounceChanges
	}
	base := visualizationir.VisualizationSpecBase{
		Title: title, Datasets: []visualizationir.VisualizationDatasetSchema{{ID: "primary", Fields: fields}},
		DataBudget:    visualizationir.VisualizationDataBudget{MaxRows: compiledVisualFrameLimit(authored, shape), RequiredCompleteness: completeness},
		Accessibility: accessibility, Interactions: customVisualizationInteractions(authored.Interaction.PointSelection),
	}
	ref := func(field string) visualizationir.VisualizationFieldRef {
		return visualizationir.VisualizationFieldRef{Dataset: "primary", Field: field}
	}
	optionalRef := func(field string) *visualizationir.VisualizationFieldRef {
		for _, column := range columns {
			if column == field {
				value := ref(field)
				return &value
			}
		}
		return nil
	}
	presentation := authored.Presentation
	common := visualizationir.VisualizationPresentation{Legend: compiledLegend(presentation.Legend), ShowLabels: presentation.ShowLabels}

	switch authored.Type {
	case "kpi":
		base.Kind = "kpi"
		return visualizationir.VisualizationSpec{Value: &visualizationir.KPIVisualizationSpec{
			VisualizationSpecBase: base, Kind: "kpi", Value: ref("value"),
			Presentation: visualizationir.KPIVisualizationPresentation{Trend: compiledKPITrend(presentation.Tone), Note: optionalString(presentation.Note), Tone: compiledTone(presentation.Tone), Thresholds: compiledThresholds(presentation.Thresholds)},
		}}, nil
	case "pie", "donut", "funnel":
		base.Kind = "proportional"
		return visualizationir.VisualizationSpec{Value: &visualizationir.ProportionalVisualizationSpec{
			VisualizationSpecBase: base, Kind: "proportional", Mark: visualizationir.VisualizationProportionalMark(authored.Type), Category: ref("label"), Value: ref("value"), Series: optionalRef("series"),
			Presentation: visualizationir.ProportionalVisualizationPresentation{VisualizationPresentation: common, Orientation: compiledOrientation(presentation.Orientation), Rose: presentation.Rose, CenterLabel: optionalString(presentation.CenterLabel), LabelPosition: compiledLabelPosition(presentation.LabelPosition), InnerRadius: optionalPositiveFloat(presentation.InnerRadius), OuterRadius: optionalPositiveFloat(presentation.OuterRadius), Align: optionalString(presentation.Align), Sort: compiledSortDirection(presentation.Sort)},
		}}, nil
	case "treemap", "sunburst", "tree", "sankey", "graph":
		base.Kind = "hierarchy"
		return visualizationir.VisualizationSpec{Value: &visualizationir.HierarchyVisualizationSpec{
			VisualizationSpecBase: base, Kind: "hierarchy", Mark: visualizationir.VisualizationHierarchyMark(authored.Type), Node: ref(firstCompiledField(columns, "node", "source", "label")), Value: optionalRef("value"), Parent: optionalRef("parent"), Source: optionalRef("source"), Target: optionalRef("target"),
			Presentation: visualizationir.HierarchyVisualizationPresentation{VisualizationPresentation: common, Orientation: compiledOrientation(presentation.Orientation), InitialDepth: optionalPositiveInt32(presentation.InitialDepth), Roam: presentation.Roam, Layout: compiledHierarchyLayout(presentation.Layout), Breadcrumb: presentation.Breadcrumb, NodeGap: optionalPositiveFloat(presentation.NodeGap), Curveness: optionalPositiveFloat(presentation.Curveness), Focus: compiledGraphFocus(presentation.Focus)},
		}}, nil
	case "radar", "gauge":
		base.Kind = "polar"
		return visualizationir.VisualizationSpec{Value: &visualizationir.PolarVisualizationSpec{
			VisualizationSpecBase: base, Kind: "polar", Mark: visualizationir.VisualizationPolarMark(authored.Type), Category: optionalRef("label"), Value: ref("value"), Series: optionalRef("series"),
			Presentation: visualizationir.PolarVisualizationPresentation{VisualizationPresentation: common, Minimum: presentation.Minimum, Maximum: presentation.Maximum, Target: presentation.Target, ShowPointer: true, Area: presentation.Area, ProgressWidth: optionalPositiveFloat(presentation.ProgressWidth), Thresholds: compiledThresholds(presentation.Thresholds)},
		}}, nil
	default:
		mark := visualizationir.VisualizationCartesianMark(authored.Type)
		supported := map[visualizationir.VisualizationCartesianMark]bool{
			visualizationir.VisualizationCartesianMarkLine: true, visualizationir.VisualizationCartesianMarkArea: true, visualizationir.VisualizationCartesianMarkBar: true,
			visualizationir.VisualizationCartesianMarkColumn: true, visualizationir.VisualizationCartesianMarkScatter: true, visualizationir.VisualizationCartesianMarkHistogram: true,
			visualizationir.VisualizationCartesianMarkCombo: true, visualizationir.VisualizationCartesianMarkWaterfall: true, visualizationir.VisualizationCartesianMarkCandlestick: true,
			visualizationir.VisualizationCartesianMarkBoxplot: true, visualizationir.VisualizationCartesianMarkHeatmap: true,
		}
		if !supported[mark] {
			return visualizationir.VisualizationSpec{}, fmt.Errorf("unsupported visualization type %q", authored.Type)
		}
		base.Kind = "cartesian"
		xField := firstCompiledField(columns, "label", "row", "name")
		y := make([]visualizationir.VisualizationFieldRef, 0, len(columns))
		for _, column := range columns {
			if column != xField && column != "series" && column != "selected" && column != "positive" {
				y = append(y, ref(column))
			}
		}
		if len(y) == 0 {
			y = append(y, ref("value"))
		}
		showSymbols := true
		if presentation.ShowSymbols != nil {
			showSymbols = *presentation.ShowSymbols
		}
		area := authored.Type == "area"
		if presentation.Area != nil && *presentation.Area {
			area = true
		}
		decisionContext, err := compileCartesianDecisionContext(columns, presentation)
		if err != nil {
			return visualizationir.VisualizationSpec{}, err
		}
		return visualizationir.VisualizationSpec{Value: &visualizationir.CartesianVisualizationSpec{
			VisualizationSpecBase: base, Kind: "cartesian", Mark: mark, X: ref(xField), Y: y, Series: optionalRef("series"),
			Axes: decisionContext.axes, ReferenceLines: decisionContext.referenceLines, ReferenceBands: decisionContext.referenceBands,
			EventAnnotations: decisionContext.eventAnnotations, Tooltip: decisionContext.tooltip,
			Presentation: visualizationir.CartesianVisualizationPresentation{VisualizationPresentation: common, Smooth: presentation.Smooth, Stacked: presentation.Stacked, ShowSymbols: showSymbols, DataZoom: presentation.DataZoom, Area: area, Step: presentation.Step, Orientation: compiledOptionalOrientation(presentation.Orientation), LabelPosition: compiledLabelPosition(presentation.LabelPosition), SymbolSize: optionalPositiveFloat(presentation.SymbolSize), HistogramBins: optionalPositiveInt32(presentation.HistogramBins), ComboSeries: compiledComboSeries(presentation.SeriesTypes, presentation.DualAxis)},
		}}, nil
	}
}

type compiledCartesianDecisionContextValue struct {
	axes             *[]visualizationir.VisualizationAxisConfiguration
	referenceLines   *[]visualizationir.VisualizationReferenceLine
	referenceBands   *[]visualizationir.VisualizationReferenceBand
	eventAnnotations *[]visualizationir.VisualizationEventAnnotation
	tooltip          *[]visualizationir.VisualizationFieldRef
}

func compileCartesianDecisionContext(columns []string, presentation reportdef.VisualPresentation) (compiledCartesianDecisionContextValue, error) {
	var result compiledCartesianDecisionContextValue
	if len(presentation.Axes) > 0 {
		values := make([]visualizationir.VisualizationAxisConfiguration, len(presentation.Axes))
		for index, authored := range presentation.Axes {
			values[index] = visualizationir.VisualizationAxisConfiguration{
				ID:          visualizationir.VisualizationCartesianAxis(authored.ID),
				Title:       optionalString(authored.Title),
				Scale:       compiledAxisScale(authored.Scale),
				Zero:        compiledAxisZeroPolicy(authored.Zero),
				Minimum:     authored.Minimum,
				Maximum:     authored.Maximum,
				Unit:        optionalString(authored.Unit),
				TickDensity: compiledAxisTickDensity(authored.TickDensity),
			}
		}
		result.axes = &values
	}
	compileValue := func(authored reportdef.VisualReferenceValue) (visualizationir.VisualizationReferenceValue, error) {
		switch {
		case authored.Number != nil:
			return visualizationir.VisualizationReferenceValue{Value: &visualizationir.NumberVisualizationReferenceValue{
				VisualizationReferenceValueBase: visualizationir.VisualizationReferenceValueBase{Kind: "number"},
				Kind:                            "number",
				Value:                           *authored.Number,
			}}, nil
		case authored.Text != "":
			return visualizationir.VisualizationReferenceValue{Value: &visualizationir.TextVisualizationReferenceValue{
				VisualizationReferenceValueBase: visualizationir.VisualizationReferenceValueBase{Kind: "text"},
				Kind:                            "text",
				Value:                           authored.Text,
			}}, nil
		case authored.Field != "":
			if !containsCompiledColumn(columns, authored.Field) {
				return visualizationir.VisualizationReferenceValue{}, fmt.Errorf("reference field %q is not in the compiled result", authored.Field)
			}
			reducer := authored.Reducer
			if reducer == "" {
				reducer = "first"
			}
			return visualizationir.VisualizationReferenceValue{Value: &visualizationir.FieldVisualizationReferenceValue{
				VisualizationReferenceValueBase: visualizationir.VisualizationReferenceValueBase{Kind: "field"},
				Kind:                            "field",
				Field:                           visualizationir.VisualizationFieldRef{Dataset: "primary", Field: authored.Field},
				Reducer:                         visualizationir.VisualizationReferenceReducer(reducer),
			}}, nil
		default:
			return visualizationir.VisualizationReferenceValue{}, fmt.Errorf("reference value requires number, text, or field")
		}
	}
	tone := func(value string) visualizationir.VisualizationTone {
		if value == "" {
			return visualizationir.VisualizationToneNeutral
		}
		return visualizationir.VisualizationTone(value)
	}
	if len(presentation.ReferenceLines) > 0 {
		values := make([]visualizationir.VisualizationReferenceLine, len(presentation.ReferenceLines))
		for index, authored := range presentation.ReferenceLines {
			value, err := compileValue(authored.Value)
			if err != nil {
				return result, fmt.Errorf("reference line %q: %w", authored.ID, err)
			}
			values[index] = visualizationir.VisualizationReferenceLine{
				ID: authored.ID, Axis: visualizationir.VisualizationCartesianAxis(authored.Axis), Value: value,
				Label: optionalString(authored.Label), Tone: tone(authored.Tone),
			}
		}
		result.referenceLines = &values
	}
	if len(presentation.ReferenceBands) > 0 {
		values := make([]visualizationir.VisualizationReferenceBand, len(presentation.ReferenceBands))
		for index, authored := range presentation.ReferenceBands {
			from, err := compileValue(authored.From)
			if err != nil {
				return result, fmt.Errorf("reference band %q from: %w", authored.ID, err)
			}
			to, err := compileValue(authored.To)
			if err != nil {
				return result, fmt.Errorf("reference band %q to: %w", authored.ID, err)
			}
			values[index] = visualizationir.VisualizationReferenceBand{
				ID: authored.ID, Axis: visualizationir.VisualizationCartesianAxis(authored.Axis), From: from, To: to,
				Label: optionalString(authored.Label), Tone: tone(authored.Tone),
			}
		}
		result.referenceBands = &values
	}
	if len(presentation.EventAnnotations) > 0 {
		values := make([]visualizationir.VisualizationEventAnnotation, len(presentation.EventAnnotations))
		for index, authored := range presentation.EventAnnotations {
			value, err := compileValue(authored.Value)
			if err != nil {
				return result, fmt.Errorf("event annotation %q: %w", authored.ID, err)
			}
			values[index] = visualizationir.VisualizationEventAnnotation{
				ID: authored.ID, Axis: visualizationir.VisualizationCartesianAxis(authored.Axis), Value: value,
				Label: authored.Label, Description: optionalString(authored.Description), Tone: tone(authored.Tone),
			}
		}
		result.eventAnnotations = &values
	}
	if len(presentation.Tooltip) > 0 {
		values := make([]visualizationir.VisualizationFieldRef, len(presentation.Tooltip))
		for index, field := range presentation.Tooltip {
			if !containsCompiledColumn(columns, field) {
				return result, fmt.Errorf("tooltip field %q is not in the compiled result", field)
			}
			values[index] = visualizationir.VisualizationFieldRef{Dataset: "primary", Field: field}
		}
		result.tooltip = &values
	}
	return result, nil
}

func compiledAxisScale(value string) visualizationir.VisualizationAxisScale {
	if value == "" {
		return visualizationir.VisualizationAxisScaleAutomatic
	}
	return visualizationir.VisualizationAxisScale(value)
}

func compiledAxisZeroPolicy(value string) visualizationir.VisualizationAxisZeroPolicy {
	if value == "" {
		return visualizationir.VisualizationAxisZeroPolicyAutomatic
	}
	return visualizationir.VisualizationAxisZeroPolicy(value)
}

func compiledAxisTickDensity(value string) visualizationir.VisualizationAxisTickDensity {
	if value == "" {
		return visualizationir.VisualizationAxisTickDensityAutomatic
	}
	return visualizationir.VisualizationAxisTickDensity(value)
}

func containsCompiledColumn(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func compiledShapeColumns(shape string) []string {
	columns := map[string][]string{
		"single_value": {"label", "value", "series"}, "category_value": {"label", "value"}, "category_series_value": {"label", "series", "value"},
		"category_multi_measure": {"label", "series", "value"}, "category_delta": {"label", "value", "start", "end", "positive"},
		"binned_measure": {"label", "binStart", "binEnd", "value"}, "hierarchy": {"node", "parent", "value"}, "matrix": {"row", "column", "value"},
		"graph": {"source", "target", "value"}, "ohlc": {"label", "open", "close", "low", "high"}, "distribution": {"label", "min", "q1", "median", "q3", "max"},
	}[shape]
	return append([]string(nil), columns...)
}

func compiledShapeMeasure(field string) bool {
	switch field {
	case "value", "start", "end", "binStart", "binEnd", "open", "close", "low", "high", "min", "q1", "median", "q3", "max":
		return true
	default:
		return false
	}
}

func compiledShapeDataType(field string) visualizationir.VisualizationDataType {
	if compiledShapeMeasure(field) {
		return visualizationir.VisualizationDataTypeDecimal
	}
	if field == "positive" {
		return visualizationir.VisualizationDataTypeBoolean
	}
	return visualizationir.VisualizationDataTypeString
}

func compiledShapeLabel(field string) string {
	labels := map[string]string{"binStart": "Bin Start", "binEnd": "Bin End", "q1": "Q1", "q3": "Q3"}
	if label := labels[field]; label != "" {
		return label
	}
	if field == "" {
		return "Value"
	}
	return strings.ToUpper(field[:1]) + field[1:]
}

func firstCompiledField(columns []string, candidates ...string) string {
	for _, candidate := range candidates {
		for _, column := range columns {
			if candidate == column {
				return candidate
			}
		}
	}
	if len(columns) > 0 {
		return columns[0]
	}
	return "value"
}

func compiledLegend(value string) visualizationir.VisualizationLegendPosition {
	switch value {
	case "hidden":
		return visualizationir.VisualizationLegendPositionHidden
	case "top":
		return visualizationir.VisualizationLegendPositionTop
	case "right":
		return visualizationir.VisualizationLegendPositionRight
	case "left":
		return visualizationir.VisualizationLegendPositionLeft
	default:
		return visualizationir.VisualizationLegendPositionBottom
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalPositiveFloat(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func optionalPositiveInt32(value int) *int32 {
	if value <= 0 {
		return nil
	}
	out := int32(value)
	return &out
}

func compiledOrientation(value string) visualizationir.VisualizationOrientation {
	if value == "horizontal" {
		return visualizationir.VisualizationOrientationHorizontal
	}
	return visualizationir.VisualizationOrientationVertical
}

func compiledOptionalOrientation(value string) *visualizationir.VisualizationOrientation {
	if value == "" {
		return nil
	}
	out := compiledOrientation(value)
	return &out
}

func compiledLabelPosition(value string) *visualizationir.VisualizationLabelPosition {
	if value == "" {
		return nil
	}
	out := visualizationir.VisualizationLabelPosition(value)
	return &out
}

func compiledHierarchyLayout(value string) *visualizationir.VisualizationHierarchyLayout {
	if value == "" {
		return nil
	}
	out := visualizationir.VisualizationHierarchyLayout(value)
	return &out
}

func compiledGraphFocus(value string) *visualizationir.VisualizationGraphFocus {
	if value == "" {
		return nil
	}
	out := visualizationir.VisualizationGraphFocus(value)
	return &out
}

func compiledSortDirection(value string) *visualizationir.VisualizationSortDirection {
	if value == "" {
		return nil
	}
	out := visualizationir.VisualizationSortDirection(value)
	return &out
}

func compiledTone(value string) *visualizationir.VisualizationTone {
	if value == "" {
		return nil
	}
	out := visualizationir.VisualizationTone(value)
	return &out
}

func compiledKPITrend(value string) visualizationir.VisualizationKPITrend {
	switch value {
	case "success", "positive":
		return visualizationir.VisualizationKPITrendPositive
	case "danger", "negative":
		return visualizationir.VisualizationKPITrendNegative
	default:
		return visualizationir.VisualizationKPITrendNeutral
	}
}

func compiledThresholds(values []reportdef.VisualThreshold) *[]visualizationir.VisualizationThreshold {
	if len(values) == 0 {
		return nil
	}
	out := make([]visualizationir.VisualizationThreshold, len(values))
	for index, value := range values {
		out[index] = visualizationir.VisualizationThreshold{Value: value.Value, Tone: visualizationir.VisualizationTone(value.Tone)}
	}
	return &out
}

func compiledComboSeries(values map[string]string, dualAxis bool) *[]visualizationir.VisualizationComboSeries {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]visualizationir.VisualizationComboSeries, len(keys))
	for index, key := range keys {
		axis := visualizationir.VisualizationAxisPrimary
		if dualAxis && index > 0 {
			axis = visualizationir.VisualizationAxisSecondary
		}
		out[index] = visualizationir.VisualizationComboSeries{SeriesValue: key, Mark: visualizationir.VisualizationCartesianMark(values[key]), Axis: axis}
	}
	return &out
}

func applyCompiledSpecContract(spec *visualizationir.VisualizationSpec, authored reportdef.Visual) {
	base, err := spec.Base()
	if err != nil {
		return
	}
	base.DataBudget.MaxRows = compiledVisualLimit(authored)
	if authored.DataBudget.RequiredCompleteness != "" {
		base.DataBudget.RequiredCompleteness = visualizationir.VisualizationCompleteness(authored.DataBudget.RequiredCompleteness)
	}
	if authored.Accessibility.Title != "" {
		base.Accessibility.Title = authored.Accessibility.Title
	}
	if authored.Accessibility.Description != "" {
		base.Accessibility.Description = authored.Accessibility.Description
	}
	if authored.Accessibility.Summary != "" {
		base.Accessibility.Summary = &authored.Accessibility.Summary
	}
	if authored.Accessibility.AnnounceChanges {
		base.Accessibility.AnnounceChanges = &authored.Accessibility.AnnounceChanges
	}
}
