package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/workspace"
)

func TestCompileOlistWorkspace(t *testing.T) {
	compiled, err := Compile(filepath.Join("..", "..", "..", "dashboards", "catalog.yaml"), Options{
		WorkspaceID:  "libredash",
		DeploymentID: "dep_test",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v, want Olist workspace to compile", err)
	}
	if compiled.Definition == nil {
		t.Fatal("Compile() returned nil workspace definition")
	}
	if compiled.Workspace.ID != "libredash" {
		t.Fatalf("workspace id = %q, want libredash", compiled.Workspace.ID)
	}
	if len(compiled.Workspace.Graph.Assets) == 0 {
		t.Fatal("expected compiled asset graph")
	}
}

func TestCompileRejectsBadSemanticReferences(t *testing.T) {
	catalogPath := writeCompilerWorkspace(t, `
id: sales
title: Sales
semantic_model: olist
filters:
  missing:
    type: multi_select
    label: Missing
    url_param: missing
    operator: in
    field: orders.missing
visuals:
  revenue:
    kind: kpi
    query:
      measures:
        revenue:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`)

	_, err := Compile(catalogPath, Options{WorkspaceID: "libredash", DeploymentID: "dep_test"})
	if err == nil {
		t.Fatal("Compile() error = nil, want bad semantic reference failure")
	}
	if !strings.Contains(err.Error(), "unknown dimension") {
		t.Fatalf("Compile() error = %v, want unknown dimension failure", err)
	}
}

func TestCompileRejectsLegacyVocabulary(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantText string
	}{
		{
			name: "metric_views",
			files: map[string]string{
				"catalog.yaml": `
workspace:
  id: libredash
  title: LibreDash Workspace
metric_views: []
semantic_models:
  - id: olist
    title: Olist
    path: model.yaml
dashboards:
  - id: sales
    title: Sales
    path: dashboard.yaml
`,
				"model.yaml":     validCompilerModelYAML(),
				"dashboard.yaml": validCompilerDashboardYAML(),
			},
			wantText: "metric views",
		},
		{
			name: "dataset",
			files: map[string]string{
				"catalog.yaml":   validCompilerCatalogYAML(),
				"model.yaml":     validCompilerModelYAML() + "\ndatasets: {}\n",
				"dashboard.yaml": validCompilerDashboardYAML(),
			},
			wantText: "datasets",
		},
		{
			name: "cache_table",
			files: map[string]string{
				"catalog.yaml":   validCompilerCatalogYAML(),
				"model.yaml":     validCompilerModelYAML() + "\ncache_tables: {}\n",
				"dashboard.yaml": validCompilerDashboardYAML(),
			},
			wantText: "cache_tables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeCompilerFixture(t, filepath.Join(dir, name), content)
			}
			_, err := Compile(filepath.Join(dir, "catalog.yaml"), Options{WorkspaceID: "libredash", DeploymentID: "dep_test"})
			if err == nil {
				t.Fatal("Compile() error = nil, want legacy vocabulary rejection")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("Compile() error = %v, want text %q", err, tt.wantText)
			}
		})
	}
}

func TestCompileLineageAssetTypes(t *testing.T) {
	compiled, err := Compile(filepath.Join("..", "..", "..", "dashboards", "catalog.yaml"), Options{
		WorkspaceID:  "libredash",
		DeploymentID: "dep_test",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v, want Olist workspace to compile", err)
	}

	types := map[workspace.AssetType]bool{}
	for _, asset := range compiled.Workspace.Graph.Assets {
		types[asset.Type] = true
	}
	for _, want := range []workspace.AssetType{
		workspace.AssetTypeSemanticModel,
		workspace.AssetTypeModelTable,
		workspace.AssetTypeSemanticTable,
		workspace.AssetTypeRelationship,
		workspace.AssetTypeMeasure,
		workspace.AssetTypeSource,
		workspace.AssetTypeConnection,
		workspace.AssetTypeDashboard,
		workspace.AssetTypePage,
		workspace.AssetTypePageItem,
		workspace.AssetTypeVisual,
		workspace.AssetTypeFilter,
		workspace.AssetTypeTable,
	} {
		if !types[want] {
			t.Fatalf("lineage asset type %q missing: %#v", want, types)
		}
	}
	for _, notWant := range []workspace.AssetType{"metric_view", "dataset", "cache_table"} {
		if types[notWant] {
			t.Fatalf("legacy asset type %q should not be present: %#v", notWant, types)
		}
	}

	edgeTypes := map[workspace.AssetEdgeType]bool{}
	for _, edge := range compiled.Workspace.Graph.Edges {
		edgeTypes[edge.Type] = true
	}
	for _, notWant := range []workspace.AssetEdgeType{"uses_metric_view", "uses_dataset", "uses_cache_table"} {
		if edgeTypes[notWant] {
			t.Fatalf("legacy edge type %q should not be present: %#v", notWant, edgeTypes)
		}
	}
}

func writeCompilerWorkspace(t *testing.T, dashboardYAML string) string {
	t.Helper()
	dir := t.TempDir()
	writeCompilerFixture(t, filepath.Join(dir, "catalog.yaml"), validCompilerCatalogYAML())
	writeCompilerFixture(t, filepath.Join(dir, "model.yaml"), validCompilerModelYAML())
	writeCompilerFixture(t, filepath.Join(dir, "dashboard.yaml"), dashboardYAML)
	return filepath.Join(dir, "catalog.yaml")
}

func validCompilerCatalogYAML() string {
	return `
workspace:
  id: libredash
  title: LibreDash Workspace
semantic_models:
  - id: olist
    title: Olist
    path: model.yaml
dashboards:
  - id: sales
    title: Sales
    path: dashboard.yaml
`
}

func validCompilerModelYAML() string {
	return `
name: olist
title: Olist
connections:
  olist:
    kind: local
sources:
  orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    source: orders
    primary_key: order_id
    fields:
      status: {label: Status}
      revenue: {label: Revenue}
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue), format: currency}
`
}

func validCompilerDashboardYAML() string {
	return `
id: sales
title: Sales
semantic_model: olist
filters: {}
visuals:
  revenue:
    kind: kpi
    query:
      measures:
        revenue:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`
}

func writeCompilerFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
