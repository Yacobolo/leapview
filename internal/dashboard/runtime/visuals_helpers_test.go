package runtime

import (
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
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
