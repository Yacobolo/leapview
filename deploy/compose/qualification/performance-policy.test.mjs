import assert from 'node:assert/strict'
import test from 'node:test'

import {
  comparePerformance,
  evaluatePerformance,
  parsePrometheusMetrics,
  percentile,
  summarizeDurations,
  validatePerformancePolicy,
} from './performance-policy.mjs'

const policy = {
  schemaVersion: 1,
  workload: 'olist-evaluation',
  assumptions: {
    minimumLogicalCPUs: 2,
    minimumMemoryBytes: 4_294_967_296,
    dataset: { name: 'bundled synthetic Olist evaluator', orders: 24 },
    samples: {
      coldDashboardLoads: 3,
      warmDashboardLoads: 5,
      filterInteractions: 8,
      tableInteractions: 6,
      governedQueries: 10,
      refreshRuns: 3,
      concurrentReaders: 8,
    },
  },
  budgets: {
    coldDashboardReadyP95Ms: 15_000,
    warmDashboardReadyP95Ms: 5_000,
    filterToSettleP95Ms: 3_000,
    tableInteractionP95Ms: 1_000,
    governedQueryP95Ms: 1_000,
    refreshP95Ms: 15_000,
    concurrentQueryP95Ms: 5_000,
    errorRateMax: 0,
    peakResidentMemoryBytes: 1_610_612_736,
    cpuSecondsMax: 60,
    temporaryDiskGrowthBytesMax: 67_108_864,
    goroutineGrowthMax: 25,
    openConnectionsMax: 16,
  },
  comparison: {
    maxRegressionRatio: 1.25,
    minimumMeaningfulLatencyDeltaMs: 50,
  },
}

test('percentiles use nearest-rank values and retain the full sample count', () => {
  assert.equal(percentile([5, 1, 3, 2, 4], 50), 3)
  assert.equal(percentile([5, 1, 3, 2, 4], 95), 5)
  assert.deepEqual(summarizeDurations([5, 1, 3, 2, 4]), {
    samples: 5,
    p50: 3,
    p95: 5,
    max: 5,
  })
})

test('Prometheus parsing ignores comments and preserves labeled series separately', () => {
  const parsed = parsePrometheusMetrics(`
# HELP process_resident_memory_bytes Resident memory.
process_resident_memory_bytes 1234
go_goroutines 17
leapview_workload_running{class="interactive",workspace="evaluation"} 2
leapview_workload_running{class="refresh",workspace="evaluation"} 1
`)

  assert.deepEqual(parsed, {
    process_resident_memory_bytes: [1234],
    go_goroutines: [17],
    leapview_workload_running: [2, 1],
  })
})

test('absolute performance evaluation reports every breached release budget', () => {
  const failures = evaluatePerformance({
    latency: {
      coldDashboardReadyMs: { p95: 15_001 },
      warmDashboardReadyMs: { p95: 5_001 },
      filterToSettleMs: { p95: 3_001 },
      tableInteractionMs: { p95: 1_001 },
      governedQueryMs: { p95: 1_001 },
      refreshMs: { p95: 15_001 },
      concurrentQueryMs: { p95: 5_001 },
    },
    reliability: { requests: 100, errors: 1 },
    resources: {
      peakResidentMemoryBytes: 1_610_612_737,
      cpuSeconds: 60.01,
      temporaryDiskGrowthBytes: 67_108_865,
      goroutinesBefore: 10,
      goroutinesAfter: 36,
      peakOpenConnections: 17,
    },
  }, policy)

  assert.deepEqual(failures, [
    'cold dashboard readiness p95 15001ms exceeds 15000ms',
    'warm dashboard readiness p95 5001ms exceeds 5000ms',
    'filter-to-settle p95 3001ms exceeds 3000ms',
    'table interaction p95 1001ms exceeds 1000ms',
    'governed query p95 1001ms exceeds 1000ms',
    'refresh/materialization p95 15001ms exceeds 15000ms',
    'concurrent query p95 5001ms exceeds 5000ms',
    'request error rate 0.01 exceeds 0',
    'peak resident memory 1610612737 bytes exceeds 1610612736 bytes',
    'CPU consumption 60.01s exceeds 60s',
    'temporary disk growth 67108865 bytes exceeds 67108864 bytes',
    'steady-state goroutine growth 26 exceeds 25',
    'peak open connections 17 exceeds 16',
  ])
})

test('future candidate comparison applies both ratio and meaningful-delta tolerance', () => {
  const baseline = {
    latency: {
      coldDashboardReadyMs: { p95: 1_000 },
      warmDashboardReadyMs: { p95: 200 },
      filterToSettleMs: { p95: 100 },
      tableInteractionMs: { p95: 100 },
      governedQueryMs: { p95: 80 },
      refreshMs: { p95: 400 },
      concurrentQueryMs: { p95: 120 },
    },
  }
  const candidate = structuredClone(baseline)
  candidate.latency.coldDashboardReadyMs.p95 = 1_260
  candidate.latency.warmDashboardReadyMs.p95 = 240
  candidate.latency.filterToSettleMs.p95 = 140

  assert.deepEqual(comparePerformance(candidate, baseline, policy), [
    'cold dashboard readiness p95 regressed from 1000ms to 1260ms (1.26x, limit 1.25x)',
  ])
})

test('policy validation rejects incomplete or ambiguous release policies', () => {
  assert.deepEqual(validatePerformancePolicy(policy), [])
  assert.deepEqual(validatePerformancePolicy({
    ...policy,
    assumptions: {
      ...policy.assumptions,
      minimumLogicalCPUs: 0,
      samples: { ...policy.assumptions.samples, tableInteractions: 0 },
    },
    budgets: { ...policy.budgets, errorRateMax: -1 },
    comparison: { maxRegressionRatio: 1, minimumMeaningfulLatencyDeltaMs: -1 },
  }), [
    'assumptions.minimumLogicalCPUs must be greater than 0',
    'assumptions.samples.tableInteractions must be greater than 0',
    'budgets.errorRateMax must be between 0 and 1',
    'comparison.maxRegressionRatio must be greater than 1',
    'comparison.minimumMeaningfulLatencyDeltaMs must be at least 0',
  ])
})
