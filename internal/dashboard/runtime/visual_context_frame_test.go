package runtime

import (
	"testing"

	"github.com/flidai/leapview/internal/dashboard"
	visualizationdefinition "github.com/flidai/leapview/internal/dashboard/visualization/definition"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestFrameFromDatumsSelectsPrimarySchemaAmongContextDatasets(t *testing.T) {
	t.Parallel()

	definition := visualizationdefinition.Definition{
		ID:    "revenue",
		Query: visualizationdefinition.QueryBinding{DatasetID: "primary"},
		Spec: visualizationir.VisualizationSpec{Value: &visualizationir.CartesianVisualizationSpec{
			VisualizationSpecBase: visualizationir.VisualizationSpecBase{
				Datasets: []visualizationir.VisualizationDatasetSchema{
					{ID: "primary", Fields: []visualizationir.VisualizationField{{ID: "label"}, {ID: "value"}}},
					{ID: "context", Fields: []visualizationir.VisualizationField{{ID: "region"}}},
				},
			},
		}},
	}
	frame, err := frameFromDatums(definition, []dashboard.Datum{{"label": "Jan", "value": 42, "region": "ignored"}})
	if err != nil {
		t.Fatalf("frameFromDatums(): %v", err)
	}
	if len(frame.Columns) != 2 || frame.Columns[0] != "label" || frame.Columns[1] != "value" || len(frame.Rows) != 1 || frame.Rows[0][1] != 42 {
		t.Fatalf("frame = %#v", frame)
	}
}
