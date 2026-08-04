package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkWholeProjectCompilation(b *testing.B) {
	for _, size := range []struct {
		name       string
		workspaces int
	}{
		{name: "small", workspaces: 1},
		{name: "medium", workspaces: 10},
		{name: "large", workspaces: 50},
	} {
		b.Run(size.name, func(b *testing.B) {
			projectPath, editable := writeReconciliationBenchmarkProject(b, size.workspaces)
			for _, edit := range []struct {
				name  string
				files int
			}{
				{name: "no_edit", files: 0},
				{name: "single_resource_edit", files: 1},
				{name: "multi_resource_edit", files: max(2, size.workspaces/5)},
			} {
				b.Run(edit.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ReportMetric(float64(4+size.workspaces*4), "resources")
					for iteration := 0; iteration < b.N; iteration++ {
						b.StopTimer()
						for index := 0; index < edit.files && index < len(editable); index++ {
							appendBenchmarkRevision(b, editable[index], iteration)
						}
						b.StartTimer()
						if _, err := CompileProject(projectPath, Options{ServingStateID: "benchmark"}); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func writeReconciliationBenchmarkProject(b *testing.B, workspaces int) (string, []string) {
	b.Helper()
	root := b.TempDir()
	files := map[string]string{
		"leapview.yaml":                projectYAML(),
		"connections/olist.yaml":       connectionYAML("olist"),
		"sources/olist.orders.yaml":    sourceYAML("olist.orders", "orders.csv", "order_id"),
		"sources/olist.customers.yaml": sourceYAML("olist.customers", "customers.csv", "customer_id"),
	}
	editable := make([]string, 0, workspaces)
	for index := 0; index < workspaces; index++ {
		workspaceID := fmt.Sprintf("workspace_%03d", index)
		base := filepath.Join("workspaces", workspaceID)
		files[filepath.Join(base, "workspace.yaml")] = workspaceYAML(workspaceID)
		files[filepath.Join(base, "models", "orders.yaml")] = modelTableYAML(workspaceID, "orders", "olist.orders", "order_id", `SELECT order_id, order_status AS status FROM source."olist.orders"`)
		files[filepath.Join(base, "semantic-models", workspaceID+".yaml")] = semanticModelYAML(workspaceID, "orders", "order_count")
		dashboardPath := filepath.Join(base, "dashboards", "overview.yaml")
		files[dashboardPath] = dashboardYAML(workspaceID, "overview", workspaceID)
		editable = append(editable, filepath.Join(root, dashboardPath))
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	return filepath.Join(root, "leapview.yaml"), editable
}

func appendBenchmarkRevision(b *testing.B, path string, revision int) {
	b.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	marker := []byte(fmt.Sprintf("\n# benchmark revision %d\n", revision))
	if err := os.WriteFile(path, append(body, marker...), 0o600); err != nil {
		b.Fatal(err)
	}
}
