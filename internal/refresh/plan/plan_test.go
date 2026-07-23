package plan

import (
	"reflect"
	"testing"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	"github.com/Yacobolo/leapview/internal/refresh/artifact"
	refreshschedule "github.com/Yacobolo/leapview/internal/refresh/schedule"
)

func TestForPipelineOrdersDependenciesBeforeDependents(t *testing.T) {
	definition := &artifact.Definition{
		Models: map[string]*semanticmodel.Model{
			"sales": {
				Tables: map[string]semanticmodel.Table{
					"orders":    {ModelDependencies: []string{"customers"}},
					"customers": {},
				},
			},
		},
		Pipelines: map[string]refreshschedule.Definition{
			"daily": {SemanticModel: "sales"},
		},
	}

	got, err := ForPipeline(definition, "acme", "daily")
	if err != nil {
		t.Fatalf("plan refresh pipeline: %v", err)
	}
	if got.TargetType != "refresh_pipeline" || got.TargetID != "acme.daily" || got.ModelID != "sales" {
		t.Fatalf("unexpected plan identity: %#v", got)
	}
	if want := []string{"customers", "orders"}; !reflect.DeepEqual(got.Tables, want) {
		t.Fatalf("tables = %#v, want %#v", got.Tables, want)
	}
	if !reflect.DeepEqual(got.DependencyTables, got.Tables) {
		t.Fatalf("dependency tables = %#v, want %#v", got.DependencyTables, got.Tables)
	}
}

func TestForPipelineRejectsDependencyCycles(t *testing.T) {
	definition := &artifact.Definition{
		Models: map[string]*semanticmodel.Model{
			"sales": {
				Tables: map[string]semanticmodel.Table{
					"orders":    {ModelDependencies: []string{"customers"}},
					"customers": {ModelDependencies: []string{"orders"}},
				},
			},
		},
		Pipelines: map[string]refreshschedule.Definition{
			"daily": {SemanticModel: "sales"},
		},
	}

	if _, err := ForPipeline(definition, "acme", "daily"); err == nil {
		t.Fatal("expected dependency cycle to be rejected")
	}
}
