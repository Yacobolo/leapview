package semantic

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOlistModel(t *testing.T) {
	model, err := Load(filepath.Join("..", "..", "dashboards", "olist.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if model.Name != "olist" {
		t.Fatalf("model name = %q, want olist", model.Name)
	}
	if len(model.Sources) != 7 {
		t.Fatalf("source count = %d, want 7", len(model.Sources))
	}
	if got := model.Datasets["orders"].Source; got != "orders_enriched" {
		t.Fatalf("orders dataset source = %q, want orders_enriched", got)
	}
	if got := model.Visuals["revenue"].Dataset; got != "orders" {
		t.Fatalf("revenue visual dataset = %q, want orders", got)
	}
	if got := model.Visuals["orders"].Type; got != "donut" {
		t.Fatalf("orders visual type = %q, want donut", got)
	}
	if got := model.Visuals["orders_by_month_status"].Query.Series; got != "status" {
		t.Fatalf("multi-series visual series = %q, want status", got)
	}
	if got := model.Tables["orders"].DefaultSort.Key; got != "purchase_date" {
		t.Fatalf("orders table default sort = %q, want purchase_date", got)
	}
	if len(model.Pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(model.Pages))
	}
	if got := model.Pages[1].ID; got != "operations" {
		t.Fatalf("second page id = %q, want operations", got)
	}
	if len(model.Relationships) != 6 {
		t.Fatalf("relationship count = %d, want 6", len(model.Relationships))
	}
}

func TestValidateRejectsUnknownDatasetSource(t *testing.T) {
	model := loadOlistModel(t)
	dataset := model.Datasets["orders"]
	dataset.Source = "missing_cache"
	model.Datasets["orders"] = dataset

	assertValidateError(t, model, "unknown cache table")
}

func TestValidateRejectsUnknownVisualDimension(t *testing.T) {
	model := loadOlistModel(t)
	visual := model.Visuals["revenue"]
	visual.Query.Dimensions = []string{"missing_dimension"}
	model.Visuals["revenue"] = visual

	assertValidateError(t, model, "unknown dimension")
}

func TestValidateRejectsUnknownInteractionTarget(t *testing.T) {
	model := loadOlistModel(t)
	visual := model.Visuals["orders"]
	visual.Interaction.Targets.Visuals = append(visual.Interaction.Targets.Visuals, "missing_visual")
	model.Visuals["orders"] = visual

	assertValidateError(t, model, "unknown target visual")
}

func TestValidateRejectsSeriesOnUnsupportedChart(t *testing.T) {
	model := loadOlistModel(t)
	visual := model.Visuals["orders"]
	visual.Query.Series = "status"
	model.Visuals["orders"] = visual

	assertValidateError(t, model, "does not support series")
}

func loadOlistModel(t *testing.T) *Model {
	t.Helper()
	model, err := Load(filepath.Join("..", "..", "dashboards", "olist.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func assertValidateError(t *testing.T, model *Model, contains string) {
	t.Helper()
	err := model.Validate()
	if err == nil {
		t.Fatalf("Validate() error = nil, want %q", contains)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("Validate() error = %q, want containing %q", err.Error(), contains)
	}
}
