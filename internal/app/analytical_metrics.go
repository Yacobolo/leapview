package app

import (
	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	"github.com/prometheus/client_golang/prometheus"
)

type analyticalCollector struct {
	engines                                               *analyticsduckdb.EnginePool
	cache                                                 *resultcache.Pool
	engineOpen, engineActive, engineIdle, engineEvents    *prometheus.Desc
	cacheEntries, cacheBytes, cacheEvictions, cacheStores *prometheus.Desc
	resourceExhaustions                                   *prometheus.Desc
	engineAcquisition, resultRows, resultBytes            *prometheus.Desc
}

func newAnalyticalCollector(engines *analyticsduckdb.EnginePool, cache *resultcache.Pool) *analyticalCollector {
	return &analyticalCollector{
		engines: engines, cache: cache,
		engineOpen:          prometheus.NewDesc("leapview_duckdb_engines_open", "Open generation-scoped DuckDB engines.", nil, nil),
		engineActive:        prometheus.NewDesc("leapview_duckdb_engines_active", "Active DuckDB engine leases.", nil, nil),
		engineIdle:          prometheus.NewDesc("leapview_duckdb_engines_idle", "Idle reusable DuckDB engines.", nil, nil),
		engineEvents:        prometheus.NewDesc("leapview_duckdb_engine_events_total", "DuckDB engine lifecycle events.", []string{"event"}, nil),
		cacheEntries:        prometheus.NewDesc("leapview_query_cache_entries", "Retained governed query-result entries.", nil, nil),
		cacheBytes:          prometheus.NewDesc("leapview_query_cache_bytes", "Estimated retained governed query-result bytes.", nil, nil),
		cacheEvictions:      prometheus.NewDesc("leapview_query_cache_evictions_total", "Query-result cache evictions by limiting constraint.", []string{"constraint"}, nil),
		cacheStores:         prometheus.NewDesc("leapview_query_cache_store_total", "Query-result cache store outcomes.", []string{"outcome"}, nil),
		resourceExhaustions: prometheus.NewDesc("leapview_analytics_resource_exhaustions_total", "Analytical resource-limit failures.", []string{"resource"}, nil),
		engineAcquisition:   prometheus.NewDesc("leapview_duckdb_engine_acquisition_seconds", "DuckDB engine acquisition duration.", nil, nil),
		resultRows:          prometheus.NewDesc("leapview_analytics_result_rows", "Physical analytical result row distribution.", nil, nil),
		resultBytes:         prometheus.NewDesc("leapview_analytics_result_bytes", "Estimated physical analytical result byte distribution.", nil, nil),
	}
}
func (c *analyticalCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.engineOpen, c.engineActive, c.engineIdle, c.engineEvents, c.cacheEntries, c.cacheBytes, c.cacheEvictions, c.cacheStores, c.resourceExhaustions, c.engineAcquisition, c.resultRows, c.resultBytes} {
		ch <- d
	}
}
func (c *analyticalCollector) Collect(ch chan<- prometheus.Metric) {
	if c.engines != nil {
		s := c.engines.Stats()
		ch <- prometheus.MustNewConstMetric(c.engineOpen, prometheus.GaugeValue, float64(s.Open))
		ch <- prometheus.MustNewConstMetric(c.engineActive, prometheus.GaugeValue, float64(s.Active))
		ch <- prometheus.MustNewConstMetric(c.engineIdle, prometheus.GaugeValue, float64(s.Idle))
		for event, value := range map[string]uint64{"open": s.Opens, "reuse": s.Reuses, "eviction": s.Evictions, "initialization_failure": s.InitializationFailures, "cleanup": s.Cleanups, "cleanup_failure": s.CleanupFails} {
			ch <- prometheus.MustNewConstMetric(c.engineEvents, prometheus.CounterValue, float64(value), event)
		}
		for _, reason := range []string{"memory", "temp", "result_rows", "result_bytes"} {
			ch <- prometheus.MustNewConstMetric(c.resourceExhaustions, prometheus.CounterValue, float64(s.Exhaustions[reason]), reason)
		}
		ch <- prometheus.MustNewConstHistogram(c.engineAcquisition, s.AcquisitionDuration.Count, s.AcquisitionDuration.Sum, s.AcquisitionDuration.Buckets)
		ch <- prometheus.MustNewConstHistogram(c.resultRows, s.ResultRows.Count, s.ResultRows.Sum, s.ResultRows.Buckets)
		ch <- prometheus.MustNewConstHistogram(c.resultBytes, s.ResultBytes.Count, s.ResultBytes.Sum, s.ResultBytes.Buckets)
	}
	if c.cache != nil {
		s := c.cache.Stats()
		ch <- prometheus.MustNewConstMetric(c.cacheEntries, prometheus.GaugeValue, float64(s.Entries))
		ch <- prometheus.MustNewConstMetric(c.cacheBytes, prometheus.GaugeValue, float64(s.Bytes))
		for _, constraint := range []resultcache.Constraint{resultcache.ConstraintRuntime, resultcache.ConstraintWorkspace, resultcache.ConstraintNode} {
			ch <- prometheus.MustNewConstMetric(c.cacheEvictions, prometheus.CounterValue, float64(s.Evictions[constraint]), string(constraint))
		}
		for _, outcome := range []resultcache.StoreOutcome{resultcache.StoreStored, resultcache.StoreOversized, resultcache.StoreStale, resultcache.StoreClosed} {
			ch <- prometheus.MustNewConstMetric(c.cacheStores, prometheus.CounterValue, float64(s.Stores[outcome]), string(outcome))
		}
	}
}
