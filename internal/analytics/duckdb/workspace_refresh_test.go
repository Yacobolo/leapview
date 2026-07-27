package duckdb

import (
	"testing"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
)

func TestApplyDiscoveredSourceSchemasPreservesAuthoredMetadata(t *testing.T) {
	nullable := true
	refreshed := &semanticmodel.Model{Sources: map[string]semanticmodel.Source{
		"orders": {Schema: semanticmodel.TableSchema{Columns: []semanticmodel.ColumnSchema{{
			Name: "order_id", Ordinal: 1, PhysicalType: "VARCHAR", Nullable: &nullable,
		}}}},
	}}
	authored := &semanticmodel.Model{Sources: map[string]semanticmodel.Source{
		"orders": {Description: "Authored source", Fields: map[string]semanticmodel.SourceField{
			"order_id": {Description: "Authored field"},
		}},
	}}

	applyDiscoveredSourceSchemas(refreshed, map[string]*semanticmodel.Model{"commerce": authored})

	source := authored.Sources["orders"]
	if source.Description != "Authored source" || source.Fields["order_id"].Description != "Authored field" {
		t.Fatalf("authored metadata was replaced: %#v", source)
	}
	if len(source.Schema.Columns) != 1 || source.Schema.Columns[0].Name != "order_id" || source.Schema.Columns[0].PhysicalType != "VARCHAR" {
		t.Fatalf("discovered schema was not propagated: %#v", source.Schema)
	}
	refreshedColumn := refreshed.Sources["orders"]
	*refreshedColumn.Schema.Columns[0].Nullable = false
	if source.Schema.Columns[0].Nullable == nil || !*source.Schema.Columns[0].Nullable {
		t.Fatal("propagated source schema aliases refresh-owned metadata")
	}
}
