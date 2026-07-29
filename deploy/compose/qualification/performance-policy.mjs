const latencyPhases = [
  ['coldDashboardReadyMs', 'cold dashboard readiness', 'coldDashboardReadyP95Ms'],
  ['warmDashboardReadyMs', 'warm dashboard readiness', 'warmDashboardReadyP95Ms'],
  ['filterToSettleMs', 'filter-to-settle', 'filterToSettleP95Ms'],
  ['tableInteractionMs', 'table interaction', 'tableInteractionP95Ms'],
  ['governedQueryMs', 'governed query', 'governedQueryP95Ms'],
  ['refreshMs', 'refresh/materialization', 'refreshP95Ms'],
  ['concurrentQueryMs', 'concurrent query', 'concurrentQueryP95Ms'],
]

export function percentile(values, rank) {
  if (values.length === 0) return 0
  const sorted = [...values].sort((left, right) => left - right)
  const index = Math.max(0, Math.ceil((rank / 100) * sorted.length) - 1)
  return round(sorted[Math.min(index, sorted.length - 1)])
}

export function summarizeDurations(values) {
  return {
    samples: values.length,
    p50: percentile(values, 50),
    p95: percentile(values, 95),
    max: values.length === 0 ? 0 : round(Math.max(...values)),
  }
}

export function parsePrometheusMetrics(input) {
  const result = {}
  for (const rawLine of input.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{[^}]*\})?\s+([-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][-+]?\d+)?)$/)
    if (!match) continue
    const value = Number(match[2])
    if (!Number.isFinite(value)) continue
    ;(result[match[1]] ??= []).push(value)
  }
  return result
}

export function evaluatePerformance(report, policy) {
  const failures = []
  for (const [field, label, budgetField] of latencyPhases) {
    const actual = report.latency[field]?.p95
    const limit = policy.budgets[budgetField]
    if (actual > limit) failures.push(`${label} p95 ${actual}ms exceeds ${limit}ms`)
  }

  const errorRate = report.reliability.requests === 0
    ? 1
    : report.reliability.errors / report.reliability.requests
  if (errorRate > policy.budgets.errorRateMax) {
    failures.push(`request error rate ${round(errorRate)} exceeds ${policy.budgets.errorRateMax}`)
  }

  const resources = report.resources
  const resourceBudgets = policy.budgets
  if (resources.peakResidentMemoryBytes > resourceBudgets.peakResidentMemoryBytes) {
    failures.push(`peak resident memory ${resources.peakResidentMemoryBytes} bytes exceeds ${resourceBudgets.peakResidentMemoryBytes} bytes`)
  }
  if (resources.cpuSeconds > resourceBudgets.cpuSecondsMax) {
    failures.push(`CPU consumption ${resources.cpuSeconds}s exceeds ${resourceBudgets.cpuSecondsMax}s`)
  }
  if (resources.temporaryDiskGrowthBytes > resourceBudgets.temporaryDiskGrowthBytesMax) {
    failures.push(`temporary disk growth ${resources.temporaryDiskGrowthBytes} bytes exceeds ${resourceBudgets.temporaryDiskGrowthBytesMax} bytes`)
  }
  const goroutineGrowth = resources.goroutinesAfter - resources.goroutinesBefore
  if (goroutineGrowth > resourceBudgets.goroutineGrowthMax) {
    failures.push(`steady-state goroutine growth ${goroutineGrowth} exceeds ${resourceBudgets.goroutineGrowthMax}`)
  }
  if (resources.peakOpenConnections > resourceBudgets.openConnectionsMax) {
    failures.push(`peak open connections ${resources.peakOpenConnections} exceeds ${resourceBudgets.openConnectionsMax}`)
  }
  return failures
}

export function comparePerformance(candidate, baseline, policy) {
  const failures = []
  const ratioLimit = policy.comparison.maxRegressionRatio
  const minimumDelta = policy.comparison.minimumMeaningfulLatencyDeltaMs
  for (const [field, label] of latencyPhases) {
    const previous = baseline.latency[field]?.p95
    const current = candidate.latency[field]?.p95
    if (!(previous > 0) || !(current >= 0)) continue
    const ratio = current / previous
    if (ratio > ratioLimit && current - previous >= minimumDelta) {
      failures.push(`${label} p95 regressed from ${previous}ms to ${current}ms (${round(ratio)}x, limit ${ratioLimit}x)`)
    }
  }
  return failures
}

export function validatePerformancePolicy(policy) {
  const failures = []
  if (policy?.schemaVersion !== 1) failures.push('schemaVersion must be 1')
  if (typeof policy?.workload !== 'string' || policy.workload.trim() === '') failures.push('workload must be a non-empty string')

  for (const field of ['minimumLogicalCPUs', 'minimumMemoryBytes']) {
    if (!(policy?.assumptions?.[field] > 0)) failures.push(`assumptions.${field} must be greater than 0`)
  }
  for (const field of [
    'coldDashboardLoads',
    'warmDashboardLoads',
    'filterInteractions',
    'tableInteractions',
    'governedQueries',
    'refreshRuns',
    'concurrentReaders',
  ]) {
    if (!(policy?.assumptions?.samples?.[field] > 0)) {
      failures.push(`assumptions.samples.${field} must be greater than 0`)
    }
  }

  const positiveBudgets = [
    'coldDashboardReadyP95Ms',
    'warmDashboardReadyP95Ms',
    'filterToSettleP95Ms',
    'tableInteractionP95Ms',
    'governedQueryP95Ms',
    'refreshP95Ms',
    'concurrentQueryP95Ms',
    'peakResidentMemoryBytes',
    'cpuSecondsMax',
    'temporaryDiskGrowthBytesMax',
    'goroutineGrowthMax',
    'openConnectionsMax',
  ]
  for (const field of positiveBudgets) {
    if (!(policy?.budgets?.[field] >= 0)) failures.push(`budgets.${field} must be at least 0`)
  }
  if (!(policy?.budgets?.errorRateMax >= 0 && policy.budgets.errorRateMax <= 1)) {
    failures.push('budgets.errorRateMax must be between 0 and 1')
  }
  if (!(policy?.comparison?.maxRegressionRatio > 1)) {
    failures.push('comparison.maxRegressionRatio must be greater than 1')
  }
  if (!(policy?.comparison?.minimumMeaningfulLatencyDeltaMs >= 0)) {
    failures.push('comparison.minimumMeaningfulLatencyDeltaMs must be at least 0')
  }
  return failures
}

function round(value) {
  return Math.round(value * 100) / 100
}
