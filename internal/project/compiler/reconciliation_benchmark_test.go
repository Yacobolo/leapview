package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	projectartifact "github.com/flidai/leapview/internal/project/artifact"
)

type reconciliationBenchmarkEdit struct {
	kind      string
	path      string
	baseline  string
	alternate string
}

func (edit reconciliationBenchmarkEdit) apply(tb testing.TB, alternate bool) {
	tb.Helper()
	content := edit.baseline
	if alternate {
		content = edit.alternate
	}
	if err := os.WriteFile(edit.path, []byte(content), 0o600); err != nil {
		tb.Fatal(err)
	}
}

func BenchmarkWholeProjectCompilation(b *testing.B) {
	for _, size := range []struct {
		name       string
		workspaces int
	}{
		{name: "small", workspaces: 1},
		{name: "medium", workspaces: 10},
		{name: "large", workspaces: 30},
	} {
		b.Run(size.name, func(b *testing.B) {
			for _, scenario := range []struct {
				name  string
				edits func([]reconciliationBenchmarkEdit) []reconciliationBenchmarkEdit
			}{
				{name: "no_edit", edits: func([]reconciliationBenchmarkEdit) []reconciliationBenchmarkEdit { return nil }},
				{name: "leaf_dashboard_edit", edits: selectBenchmarkEdits("dashboard", 1)},
				{name: "workspace_access_edit", edits: selectBenchmarkEdits("access", 1)},
				{name: "shared_source_edit", edits: selectBenchmarkEdits("source", 1)},
				{name: "multi_dashboard_edit", edits: selectBenchmarkEdits("dashboard", max(2, size.workspaces/5))},
			} {
				b.Run(scenario.name, func(b *testing.B) {
					b.StopTimer()
					projectPath, available := writeReconciliationBenchmarkProject(b, size.workspaces)
					edits := scenario.edits(available)
					baseline, err := CompileProject(projectPath, Options{ServingStateID: "benchmark"})
					if err != nil {
						b.Fatal(err)
					}
					paths, err := SourceFiles(projectPath)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.ReportMetric(float64(len(paths)), "resources")
					durations := make([]time.Duration, 0, b.N)
					previous := baseline
					observed := baseline
					for iteration := 0; iteration < b.N; iteration++ {
						b.StopTimer()
						for _, edit := range edits {
							edit.apply(b, iteration%2 == 0)
						}
						b.StartTimer()
						started := time.Now()
						compiled, compileErr := CompileProject(projectPath, Options{ServingStateID: "benchmark"})
						durations = append(durations, time.Since(started))
						b.StopTimer()
						if compileErr != nil {
							b.Fatal(compileErr)
						}
						if len(edits) == 0 && compiled.Digest() != previous.Digest() {
							b.Fatalf("no-op compilation digest changed: %s -> %s", previous.Digest(), compiled.Digest())
						}
						if len(edits) > 0 && compiled.Digest() == previous.Digest() {
							b.Fatal("semantic benchmark edit did not change compiled output")
						}
						if len(edits) > 0 && iteration == 0 {
							observed = compiled
						}
						previous = compiled
						b.StartTimer()
					}
					b.StopTimer()
					changed, total := observedCompiledAssetChanges(baseline, observed)
					b.ReportMetric(float64(changed), "affected-assets")
					if total > 0 {
						b.ReportMetric(100*float64(changed)/float64(total), "affected-pct")
					}
					closure, closureTotal := estimatedAffectedAssetClosure(baseline, observed)
					b.ReportMetric(float64(closure), "closure-assets")
					if closureTotal > 0 {
						b.ReportMetric(100*float64(closure)/float64(closureTotal), "closure-pct")
					}
					reportLatencyPercentiles(b, durations)
				})
			}
		})
	}
}

func selectBenchmarkEdits(kind string, limit int) func([]reconciliationBenchmarkEdit) []reconciliationBenchmarkEdit {
	return func(available []reconciliationBenchmarkEdit) []reconciliationBenchmarkEdit {
		selected := make([]reconciliationBenchmarkEdit, 0, limit)
		for _, edit := range available {
			if edit.kind == kind {
				selected = append(selected, edit)
				if len(selected) == limit {
					break
				}
			}
		}
		return selected
	}
}

func writeReconciliationBenchmarkProject(tb testing.TB, workspaces int) (string, []reconciliationBenchmarkEdit) {
	tb.Helper()
	root := tb.TempDir()
	files := map[string]string{
		"leapview.yaml":                projectYAML(),
		"connections/olist.yaml":       connectionYAML("olist"),
		"sources/olist.orders.yaml":    sourceYAML("olist.orders", "orders.csv", "order_id"),
		"sources/olist.customers.yaml": sourceYAML("olist.customers", "customers.csv", "customer_id"),
	}
	edits := []reconciliationBenchmarkEdit{{
		kind: "source", path: filepath.Join(root, "sources", "olist.orders.yaml"),
		baseline:  files["sources/olist.orders.yaml"],
		alternate: strings.Replace(files["sources/olist.orders.yaml"], "spec:\n", "spec:\n  description: Benchmark orders source\n", 1),
	}}
	for index := 0; index < workspaces; index++ {
		workspaceID := fmt.Sprintf("workspace_%03d", index)
		base := filepath.Join("workspaces", workspaceID)
		workspace := benchmarkWorkspaceYAML(workspaceID)
		files[filepath.Join(base, "workspace.yaml")] = workspace
		files[filepath.Join(base, "models", "orders.yaml")] = modelTableYAML(workspaceID, "orders", "olist.orders", "order_id", `SELECT order_id, order_status AS status FROM source."olist.orders"`)
		files[filepath.Join(base, "semantic-models", workspaceID+".yaml")] = semanticModelYAML(workspaceID, "orders", "order_count")
		dashboardPath := filepath.Join(base, "dashboards", "overview.yaml")
		dashboard := dashboardYAML(workspaceID, "overview", workspaceID)
		files[dashboardPath] = dashboard
		groupPath := filepath.Join(base, "access", "analysts.yaml")
		group := workspaceGroupYAML(workspaceID, "analysts", fmt.Sprintf("analyst-%03d@example.com", index))
		files[groupPath] = group
		files[filepath.Join(base, "access", "analysts-viewer.yaml")] = workspaceRoleBindingGroupYAML(workspaceID, "analysts-viewer", "viewer", "analysts")
		files[filepath.Join(base, "refresh-pipelines", "daily.yaml")] = refreshPipelineYAML(workspaceID, "daily", workspaceID, []string{"0 6 * * *|UTC"})
		files[filepath.Join(base, "publications", "website.yaml")] = dashboardPublicationYAML(workspaceID, "website", "overview", "overview", []string{"https://example.com"})
		edits = append(edits,
			reconciliationBenchmarkEdit{kind: "dashboard", path: filepath.Join(root, dashboardPath), baseline: dashboard, alternate: strings.Replace(dashboard, "title: overview", "title: Overview benchmark", 1)},
			reconciliationBenchmarkEdit{kind: "access", path: filepath.Join(root, groupPath), baseline: group, alternate: strings.Replace(group, "spec:\n", "spec:\n  description: Benchmark access group\n", 1)},
		)
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			tb.Fatal(err)
		}
	}
	return filepath.Join(root, "leapview.yaml"), edits
}

func benchmarkWorkspaceYAML(name string) string {
	content := workspaceYAMLWithPublicationsAndAccess(name)
	return strings.Replace(content, "  dashboards:\n", "  refreshPipelines:\n    include:\n      - refresh-pipelines/*.yaml\n  dashboards:\n", 1)
}

func observedCompiledAssetChanges(before, after projectartifact.Project) (int, int) {
	beforeHashes := compiledAssetHashes(before)
	afterHashes := compiledAssetHashes(after)
	total := len(beforeHashes)
	if len(afterHashes) > total {
		total = len(afterHashes)
	}
	changed := 0
	seen := make(map[string]struct{}, len(beforeHashes)+len(afterHashes))
	for key, beforeHash := range beforeHashes {
		seen[key] = struct{}{}
		if afterHash, ok := afterHashes[key]; !ok || afterHash != beforeHash {
			changed++
		}
	}
	for key := range afterHashes {
		if _, ok := seen[key]; !ok {
			changed++
		}
	}
	return changed, total
}

func compiledAssetHashes(project projectartifact.Project) map[string]string {
	hashes := map[string]string{}
	for _, workspaceID := range project.WorkspaceIDs() {
		compiled, ok := project.Workspace(workspaceID)
		if !ok {
			continue
		}
		for _, asset := range compiled.Metadata().Graph.Assets {
			hashes[workspaceID+"/"+string(asset.ID)] = asset.ContentHash
		}
	}
	return hashes
}

func estimatedAffectedAssetClosure(before, after projectartifact.Project) (int, int) {
	beforeHashes := compiledAssetHashes(before)
	afterHashes := compiledAssetHashes(after)
	affected := map[string]struct{}{}
	all := map[string]struct{}{}
	for key, hash := range beforeHashes {
		all[key] = struct{}{}
		if next, ok := afterHashes[key]; !ok || next != hash {
			affected[key] = struct{}{}
		}
	}
	for key, hash := range afterHashes {
		all[key] = struct{}{}
		if previous, ok := beforeHashes[key]; !ok || previous != hash {
			affected[key] = struct{}{}
		}
	}
	reverseDependencies := map[string][]string{}
	collect := func(project projectartifact.Project) {
		for _, workspaceID := range project.WorkspaceIDs() {
			compiled, ok := project.Workspace(workspaceID)
			if !ok {
				continue
			}
			for _, edge := range compiled.Metadata().Graph.Edges {
				dependency := workspaceID + "/" + string(edge.ToAssetID)
				consumer := workspaceID + "/" + string(edge.FromAssetID)
				reverseDependencies[dependency] = append(reverseDependencies[dependency], consumer)
			}
		}
	}
	collect(before)
	collect(after)
	queue := make([]string, 0, len(affected))
	for key := range affected {
		queue = append(queue, key)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, consumer := range reverseDependencies[current] {
			if _, seen := affected[consumer]; seen {
				continue
			}
			affected[consumer] = struct{}{}
			queue = append(queue, consumer)
		}
	}
	return len(affected), len(all)
}

func reportLatencyPercentiles(b *testing.B, durations []time.Duration) {
	if len(durations) == 0 {
		return
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	b.ReportMetric(float64(durations[(len(durations)*50-1)/100])/float64(time.Millisecond), "p50-ms")
	b.ReportMetric(float64(durations[(len(durations)*95-1)/100])/float64(time.Millisecond), "p95-ms")
}

func TestReconciliationBenchmarkSemanticEditsChangeCompiledAssets(t *testing.T) {
	projectPath, edits := writeReconciliationBenchmarkProject(t, 2)
	baseline, err := CompileProject(projectPath, Options{ServingStateID: "benchmark"})
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range edits {
		edit.apply(t, true)
		compiled, compileErr := CompileProject(projectPath, Options{ServingStateID: "benchmark"})
		if compileErr != nil {
			t.Fatal(compileErr)
		}
		changed, total := observedCompiledAssetChanges(baseline, compiled)
		closure, closureTotal := estimatedAffectedAssetClosure(baseline, compiled)
		if compiled.Digest() == baseline.Digest() || changed == 0 || total == 0 || closure < changed || closureTotal != total {
			t.Fatalf("semantic edit %q did not change compiled assets", edit.path)
		}
		edit.apply(t, false)
	}
}
