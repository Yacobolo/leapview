package app

import (
	"testing"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	"github.com/prometheus/client_golang/prometheus"
)

func TestAnalyticalCollectorUsesBoundedLabels(t *testing.T) {
	engines, err := analyticsduckdb.NewEnginePool(analyticsduckdb.EnginePoolConfig{MaxOpen: 1, NodeMemoryBytes: 512 << 20, NodeTempBytes: 2 << 30, NodeThreads: 1, TempRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := resultcache.New(resultcache.Limits{RuntimeEntries: 1, RuntimeBytes: 1024, WorkspaceEntries: 1, WorkspaceBytes: 1024, NodeEntries: 1, NodeBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(newAnalyticalCollector(engines, cache))
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"leapview_duckdb_engines_open": false, "leapview_query_cache_entries": false, "leapview_query_cache_store_total": false}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
			for _, metric := range family.Metric {
				for _, label := range metric.Label {
					if label.GetName() == "workspace" || label.GetName() == "runtime" || label.GetName() == "operation" {
						t.Fatalf("unbounded label %q", label.GetName())
					}
				}
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("metric %s missing", name)
		}
	}
}
