package compiler

import (
	"testing"

	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationdefinition "github.com/flidai/leapview/internal/dashboard/visualization/definition"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestCompileAuthoringVisualizationOwnsContextDatasetsAndMetadata(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{
		Type: "line", Title: "Revenue", Subtitle: "Current scope",
		Query: reportdef.VisualQuery{
			Dimensions: []reportdef.FieldRef{{Field: "orders.month"}},
			Measures:   []reportdef.FieldRef{{Field: "revenue"}},
		},
		Datasets: map[string]reportdef.VisualQuery{
			"context": {
				Dimensions: []reportdef.FieldRef{{Field: "orders.region", Alias: "region"}},
				Measures:   []reportdef.FieldRef{{Field: "target_revenue", Alias: "target"}},
				Limit:      1,
			},
		},
		Metadata: reportdef.VisualMetadataBindings{
			Title: &reportdef.VisualTextBinding{Dataset: "context", Field: "region", Reducer: "first", Prefix: "Revenue — ", Fallback: "Revenue"},
		},
		Presentation: reportdef.VisualPresentation{ReferenceLines: []reportdef.VisualReferenceLine{{
			ID: "target", Axis: "primary_y", Value: reportdef.VisualReferenceValue{Dataset: "context", Field: "target", Reducer: "mean"},
		}}},
	}
	ctx, err := newCompileContext("revenue", "sales", authored.Type, nil)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := compileAuthoringVisualization(ctx, reportdef.ChartVisualization(authored))
	if err != nil {
		t.Fatalf("compileAuthoringVisualization(): %v", err)
	}

	if len(definition.SecondaryQueries) != 1 {
		t.Fatalf("secondary queries = %#v", definition.SecondaryQueries)
	}
	context := definition.SecondaryQueries["context"]
	if context.Kind != visualizationdefinition.QueryAggregate || context.DatasetID != "context" || context.Aggregate == nil || context.Aggregate.Limit != 1 {
		t.Fatalf("context query = %#v", context)
	}
	base, err := visualizationir.SpecificationBase(definition.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Datasets) != 2 || base.Datasets[1].ID != "context" || len(base.Datasets[1].Fields) != 2 {
		t.Fatalf("datasets = %#v", base.Datasets)
	}
	if base.Subtitle == nil || *base.Subtitle != "Current scope" || base.MetadataBindings == nil || base.MetadataBindings.Title == nil {
		t.Fatalf("metadata = subtitle %#v bindings %#v", base.Subtitle, base.MetadataBindings)
	}
	if got := base.MetadataBindings.Title.Field; got.Dataset != "context" || got.Field != "region" {
		t.Fatalf("title field = %#v", got)
	}
	line := (*definition.Spec.Value.(*visualizationir.CartesianVisualizationSpec).ReferenceLines)[0]
	field, ok := line.Value.Value.(*visualizationir.FieldVisualizationReferenceValue)
	if !ok || field.Field.Dataset != "context" || field.Field.Field != "target" {
		t.Fatalf("reference line = %#v", line)
	}
}
