package runtime

import (
	"reflect"
	"testing"

	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	"github.com/flidai/leapview/internal/dashboard"
	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationdefinition "github.com/flidai/leapview/internal/dashboard/visualization/definition"
)

func TestAggregateMemberMetadataResolvesMetricPresentation(t *testing.T) {
	model := &semanticmodel.Model{Metrics: map[string]semanticmodel.Metric{
		"tags_per_rating": {Label: "Tags per rating", Unit: "ratio", Format: "decimal"},
	}}
	got := aggregateMemberMetadata(model, "tags_per_rating")
	if got.Label != "Tags per rating" || got.Unit != "ratio" || got.Format != "decimal" {
		t.Fatalf("metric metadata = %#v", got)
	}
}

func TestCategoryMultiMeasureDatumsDecodesBundledWideRows(t *testing.T) {
	runtime := &modelRuntime{model: &semanticmodel.Model{Measures: map[string]semanticmodel.MetricMeasure{
		"rating_count": {Label: "Ratings"},
		"tag_count":    {Label: "Tags"},
	}}}
	visual := visualPlan{Measures: []visualizationdefinition.FieldBinding{{FieldID: "rating_count"}, {FieldID: "tag_count"}}}
	rows := []dashboard.Datum{{"label": "2024-01-01", "value_0": int64(8), "value_1": int64(3)}}
	got := categoryMultiMeasureDatums(runtime, visual, rows)
	want := []dashboard.Datum{
		{"label": "2024-01-01", "series": "Ratings", "value": int64(8)},
		{"label": "2024-01-01", "series": "Tags", "value": int64(3)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("datums = %#v, want %#v", got, want)
	}
}

func TestVisualSortsAppendDeterministicDimensionAndSeriesTieBreakers(t *testing.T) {
	series := visualizationdefinition.FieldBinding{FieldID: "orders.status", Alias: "status"}
	visual := visualPlan{
		Definition: visualizationdefinition.Definition{Query: visualizationdefinition.QueryBinding{ResultShape: visualizationdefinition.ResultCategorySeriesValue}},
		Dimensions: []visualizationdefinition.FieldBinding{{FieldID: "orders.purchase_month", Alias: "purchase_month"}},
		Series:     &series,
		Sort:       []visualizationdefinition.Sort{{FieldID: "value", Direction: "desc"}},
	}
	want := []reportdef.QuerySort{
		{Field: "value", Direction: "desc"},
		{Field: "label", Direction: "asc"},
		{Field: "series", Direction: "asc"},
	}
	if got := visualSorts(visual); !reflect.DeepEqual(got, want) {
		t.Fatalf("visualSorts() = %#v, want %#v", got, want)
	}
}

func TestAliasedVisualSortsAppendEveryDimensionAsTieBreaker(t *testing.T) {
	visual := visualPlan{
		Dimensions: []visualizationdefinition.FieldBinding{
			{FieldID: "orders.state", Alias: "state"},
			{FieldID: "orders.order_id", Alias: "order_id"},
		},
		Sort: []visualizationdefinition.Sort{{FieldID: "orders.state", Direction: "desc"}},
	}
	want := []reportdef.QuerySort{
		{Field: "state", Direction: "desc"},
		{Field: "order_id", Direction: "asc"},
	}
	if got := aliasedVisualSorts(visual); !reflect.DeepEqual(got, want) {
		t.Fatalf("aliasedVisualSorts() = %#v, want %#v", got, want)
	}
}
