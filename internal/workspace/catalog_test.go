package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	catalog, _, err := LoadCatalog(filepath.Join("..", "..", "dashboards", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog.SemanticModels) != 1 {
		t.Fatalf("model catalog count = %d, want 1", len(catalog.SemanticModels))
	}
	if len(catalog.Dashboards) != 1 {
		t.Fatalf("dashboard catalog count = %d, want 1", len(catalog.Dashboards))
	}
	if got := catalog.Workspace.Title; got != "LibreDash Workspace" {
		t.Fatalf("workspace title = %q, want LibreDash Workspace", got)
	}
}

func TestLoadCatalogRejectsLegacyMetricViewsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	content := `workspace:
  id: test
  title: Test
semantic_models: []
metrics_views: []
dashboards: []
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "legacy metric views") {
		t.Fatalf("LoadCatalog error = %v, want legacy metrics_views rejection", err)
	}
}

func TestCatalogValidateRejectsDuplicateIDs(t *testing.T) {
	baseDir := filepath.Join("..", "..", "dashboards")
	catalog := Catalog{
		SemanticModels: []CatalogModel{
			{ID: "olist", Title: "Olist", Path: "olist/model.yaml"},
			{ID: "olist", Title: "Olist Copy", Path: "olist/model.yaml"},
		},
		Dashboards: []CatalogDashboard{
			{ID: "executive-sales", Title: "Executive Sales", Path: "olist/executive-sales.yaml"},
		},
	}

	assertCatalogValidateError(t, catalog, baseDir, "duplicate semantic model")
}

func TestCatalogValidateRejectsMissingPath(t *testing.T) {
	baseDir := filepath.Join("..", "..", "dashboards")
	catalog := Catalog{
		SemanticModels: []CatalogModel{
			{ID: "olist", Title: "Olist", Path: "olist/missing.yaml"},
		},
		Dashboards: []CatalogDashboard{
			{ID: "executive-sales", Title: "Executive Sales", Path: "olist/executive-sales.yaml"},
		},
	}

	assertCatalogValidateError(t, catalog, baseDir, "missing.yaml")
}

func assertCatalogValidateError(t *testing.T, catalog Catalog, baseDir, contains string) {
	t.Helper()
	err := catalog.Validate(baseDir)
	if err == nil {
		t.Fatalf("Validate() error = nil, want %q", contains)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("Validate() error = %q, want containing %q", err.Error(), contains)
	}
}
