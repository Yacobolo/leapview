package ir

import (
	"math"
	"testing"
)

func TestValidateEnvelopeRejectsInvalidInlineFrames(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*VisualizationEnvelope){
		"unknown column": func(envelope *VisualizationEnvelope) {
			state := envelope.DataState.Value.(*InlineVisualizationDataState)
			state.Datasets[0].Columns[0] = "missing"
		},
		"row width mismatch": func(envelope *VisualizationEnvelope) {
			state := envelope.DataState.Value.(*InlineVisualizationDataState)
			state.Datasets[0].Rows[0] = state.Datasets[0].Rows[0][:1]
		},
		"non finite decimal": func(envelope *VisualizationEnvelope) {
			state := envelope.DataState.Value.(*InlineVisualizationDataState)
			state.Datasets[0].Rows[0][1] = math.Inf(1)
		},
		"row budget exceeded": func(envelope *VisualizationEnvelope) {
			spec := envelope.Spec.Value.(*CartesianVisualizationSpec)
			spec.DataBudget.MaxRows = 1
			state := envelope.DataState.Value.(*InlineVisualizationDataState)
			state.Datasets[0].Rows = append(state.Datasets[0].Rows, state.Datasets[0].Rows[0])
		},
	}

	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			envelope := readEnvelopeFixture(t, "cartesian-inline.json")
			mutate(&envelope)
			if err := ValidateEnvelope(envelope); err == nil {
				t.Fatal("expected invalid frame to fail")
			}
		})
	}
}

func TestValidateEnvelopeAcceptsConformanceFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cartesian-inline.json", "table-windowed.json"} {
		envelope := readEnvelopeFixture(t, name)
		if err := ValidateEnvelope(envelope); err != nil {
			t.Fatalf("ValidateEnvelope(%s): %v", name, err)
		}
	}
}

func TestValidateSpecEnforcesGeographicLayerRequirements(t *testing.T) {
	base := VisualizationSpecBase{
		Kind: "geographic", Title: "Stores",
		Datasets: []VisualizationDatasetSchema{{ID: "primary", Fields: []VisualizationField{
			{ID: "lat", Role: VisualizationFieldRoleDimension, DataType: VisualizationDataTypeDecimal, Label: "Latitude"},
			{ID: "lon", Role: VisualizationFieldRoleDimension, DataType: VisualizationDataTypeDecimal, Label: "Longitude"},
		}}},
		DataBudget:    VisualizationDataBudget{MaxRows: 100, RequiredCompleteness: VisualizationCompletenessComplete},
		Accessibility: VisualizationAccessibility{Title: "Stores", Description: "Store locations"}, Interactions: []VisualizationInteraction{},
	}
	latitude := VisualizationFieldRef{Dataset: "primary", Field: "lat"}
	longitude := VisualizationFieldRef{Dataset: "primary", Field: "lon"}
	point := VisualizationSpec{Value: &GeographicVisualizationSpec{VisualizationSpecBase: base, Kind: "geographic", Layers: []VisualizationGeographicLayer{{ID: "stores", Kind: VisualizationGeographicLayerKindPoint, Latitude: &latitude, Longitude: &longitude}}, Presentation: GeographicVisualizationPresentation{}}}
	if err := ValidateSpec(point); err != nil {
		t.Fatalf("point layer: %v", err)
	}
	point.Value.(*GeographicVisualizationSpec).Layers[0].Longitude = nil
	if err := ValidateSpec(point); err == nil {
		t.Fatal("point layer without longitude was accepted")
	}
	join := VisualizationFieldRef{Dataset: "primary", Field: "lat"}
	choropleth := VisualizationSpec{Value: &GeographicVisualizationSpec{VisualizationSpecBase: base, Kind: "geographic", Layers: []VisualizationGeographicLayer{{ID: "states", Kind: VisualizationGeographicLayerKindChoropleth, Join: &join}}, Presentation: GeographicVisualizationPresentation{}}}
	if err := ValidateSpec(choropleth); err == nil {
		t.Fatal("choropleth layer without geometry was accepted")
	}
}
