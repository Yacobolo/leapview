import type {
  VisualizationEnvelope,
  VisualizationFormat,
  VisualizationFieldRef,
  VisualizationKPIQualitativeRange,
  VisualizationKPIValueBinding,
  VisualizationReferenceReducer,
  VisualizationTone,
} from '../../../../generated/visualization'
import type { RendererContext } from '../host-controller'
import { formatDisplayValue, formatValue, resolveDisplayUnitForFormat, type ResolvedDisplayUnit } from '../format'

export type KPIChangeStatus = 'favorable' | 'unfavorable' | 'neutral' | 'unavailable'

export interface KPITrendPoint {
  label: string
  value: number
}

export interface KPIBulletRange {
  start: number
  end: number
  label: string
  tone: VisualizationTone
}

export interface KPIState {
  current?: number
  currentText: string
  comparison?: number
  comparisonText?: string
  comparisonLabel?: string
  delta?: number
  deltaText?: string
  deltaCue?: string
  changeStatus?: KPIChangeStatus
  goal?: number
  goalText?: string
  goalLabel?: string
  progress?: number
  bulletValuePosition?: number
  bulletGoalPosition?: number
  bulletMinimum?: number
  bulletMaximum?: number
  bulletRanges: KPIBulletRange[]
  rangeLabel?: string
  rangeTone?: VisualizationTone
  trend: KPITrendPoint[]
  highlightActive: boolean
  highlightAnnouncement?: string
  accessibleSummary: string
}

export function resolveKPIState(envelope: VisualizationEnvelope, context: RendererContext): KPIState {
  const spec = envelope.spec
  if (spec.kind !== 'kpi') {
    return { currentText: '—', trend: [], bulletRanges: [], highlightActive: false, accessibleSummary: 'Value unavailable.' }
  }
  const current = numericScalar(envelope, spec.value)
  const comparison = spec.comparison ? numericReduction(envelope, spec.comparison) : undefined
  const comparisonVisible = spec.comparison && (comparison !== undefined || spec.presentation.missingComparison === 'show_unavailable')
  const goal = spec.goal ? numericReduction(envelope, spec.goal) : undefined
  const displayUnit = resolveDisplayUnitForFormat(
    spec.presentation.displayUnits ?? 'auto',
    fieldFormat(envelope, spec.value),
    [current, comparison, goal],
  )
  const currentText = formatDisplayField(envelope, spec.value, current, context, displayUnit)
  const comparisonText = comparisonVisible ? formatDisplayField(envelope, spec.comparison!.field, comparison, context, displayUnit) : undefined
  const delta = current === undefined || comparison === undefined
    ? undefined
    : spec.presentation.delta === 'relative'
      ? comparison === 0 ? undefined : (current - comparison) / Math.abs(comparison)
      : current - comparison
  const changeStatus = spec.comparison
    ? delta === undefined ? 'unavailable' : changeStatusFor(delta, spec.presentation.favorableDirection)
    : undefined
  const deltaText = spec.comparison && comparisonVisible
    ? delta === undefined ? 'Unavailable' : formatDelta(envelope, spec.value, delta, spec.presentation.delta, context, displayUnit)
    : undefined
  const deltaCue = delta === undefined ? undefined : delta > 0 ? '↑' : delta < 0 ? '↓' : '•'
  const goalText = spec.goal ? formatDisplayField(envelope, spec.goal.field, goal, context, displayUnit) : undefined
  const progress = current === undefined || goal === undefined || goal <= 0 ? undefined : clamp(current / goal, 0, 1)
  const bullet = spec.presentation.mode === 'bullet'
    ? bulletGeometry(current, goal, spec.presentation.ranges)
    : undefined
  const qualitativeRange = current === undefined
    ? undefined
    : spec.presentation.ranges.find((candidate, index) =>
      (candidate.minimum === undefined || current >= candidate.minimum) &&
      (candidate.maximum === undefined || current < candidate.maximum || index === spec.presentation.ranges.length - 1 && current === candidate.maximum))
  const trend = spec.trend ? trendPoints(envelope, spec.trend.category, spec.trend.value) : []

  const summary = [`Current ${formatField(envelope, spec.value, current, context)}.`]
  if (spec.comparison && comparisonVisible) {
    summary.push(`${spec.comparison.label} ${formatField(envelope, spec.comparison.field, comparison, context)}.`)
    const distinctChangeStatus = changeStatus && changeStatus.toLowerCase() !== deltaText?.toLowerCase()
      ? changeStatus
      : undefined
    summary.push(`Change ${deltaText}${distinctChangeStatus ? `, ${distinctChangeStatus}` : ''}.`)
  }
  if (spec.goal) summary.push(`${spec.goal.label} ${formatField(envelope, spec.goal.field, goal, context)}.`)
  if (qualitativeRange) summary.push(`Status ${qualitativeRange.label}.`)
  else if (spec.presentation.ranges.length > 0 && current !== undefined) summary.push('Status out of range.')
  if (trend.length > 1) summary.push(`Trend includes ${trend.length} points, from ${trend[0]!.label} to ${trend.at(-1)!.label}.`)
  const highlights = envelope.highlights ?? []
  const highlightActive = highlights.length > 0
  const highlightAnnouncement = highlightActive
    ? `${highlights.map((highlight) => highlight.label).filter(Boolean).join(' · ') || 'Selection'} highlighted. Comparison total is unchanged.`
    : undefined
  if (highlightAnnouncement) summary.push(highlightAnnouncement)

  return {
    ...(current === undefined ? {} : { current }),
    currentText,
    ...(comparison === undefined ? {} : { comparison }),
    ...(comparisonText === undefined ? {} : { comparisonText }),
    ...(spec.comparison && comparisonVisible ? { comparisonLabel: spec.comparison.label } : {}),
    ...(delta === undefined ? {} : { delta }),
    ...(deltaText === undefined ? {} : { deltaText }),
    ...(deltaCue === undefined ? {} : { deltaCue }),
    ...(changeStatus === undefined ? {} : { changeStatus }),
    ...(goal === undefined ? {} : { goal }),
    ...(goalText === undefined ? {} : { goalText }),
    ...(spec.goal ? { goalLabel: spec.goal.label } : {}),
    ...(progress === undefined ? {} : { progress }),
    ...(bullet?.valuePosition === undefined ? {} : { bulletValuePosition: bullet.valuePosition }),
    ...(bullet?.goalPosition === undefined ? {} : { bulletGoalPosition: bullet.goalPosition }),
    ...(bullet ? { bulletMinimum: bullet.minimum, bulletMaximum: bullet.maximum } : {}),
    bulletRanges: bullet?.ranges ?? [],
    ...(qualitativeRange ? { rangeLabel: qualitativeRange.label, rangeTone: qualitativeRange.tone } : {}),
    trend,
    highlightActive,
    ...(highlightAnnouncement ? { highlightAnnouncement } : {}),
    accessibleSummary: summary.join(' '),
  }
}

export function bulletGeometry(
  current: number | undefined,
  goal: number | undefined,
  ranges: VisualizationKPIQualitativeRange[],
): {
  minimum: number
  maximum: number
  valuePosition?: number
  goalPosition?: number
  ranges: KPIBulletRange[]
} {
  const candidates = [0, current, goal, ...ranges.flatMap((range) => [range.minimum, range.maximum])]
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  let minimum = Math.min(...candidates)
  let maximum = Math.max(...candidates)
  if (minimum === maximum) maximum = minimum + 1
  if (ranges.at(-1)?.maximum === undefined) {
    maximum += (maximum - minimum) * 0.1
  }
  const position = (value: number | undefined) => value === undefined ? undefined : clamp((value - minimum) / (maximum - minimum), 0, 1)
  const normalizedRanges = ranges.flatMap((range) => {
    const start = position(range.minimum ?? minimum)!
    const end = position(range.maximum ?? maximum)!
    return end <= start ? [] : [{ start, end, label: range.label, tone: range.tone }]
  })
  return {
    minimum,
    maximum,
    ...(current === undefined ? {} : { valuePosition: position(current) }),
    ...(goal === undefined ? {} : { goalPosition: position(goal) }),
    ranges: normalizedRanges,
  }
}

export function kpiSparklinePath(points: KPITrendPoint[], width = 100, height = 28): string {
  if (points.length === 0) return ''
  const values = points.map((point) => point.value)
  const minimum = Math.min(...values)
  const maximum = Math.max(...values)
  const span = maximum - minimum
  return points.map((point, index) => {
    const x = points.length === 1 ? width / 2 : index / (points.length - 1) * width
    const y = span === 0 ? height / 2 : height - (point.value - minimum) / span * height
    return `${index === 0 ? 'M' : 'L'}${roundCoordinate(x)},${roundCoordinate(y)}`
  }).join(' ')
}

function numericReduction(envelope: VisualizationEnvelope, binding: VisualizationKPIValueBinding): number | undefined {
  const values = numericValues(envelope, binding.field)
  if (values.length === 0) return undefined
  return reduce(values, binding.reducer)
}

function numericScalar(envelope: VisualizationEnvelope, ref: VisualizationFieldRef): number | undefined {
  return numericValues(envelope, ref)[0]
}

function numericValues(envelope: VisualizationEnvelope, ref: VisualizationFieldRef): number[] {
  if (envelope.dataState.kind !== 'inline') return []
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === ref.dataset)
  const index = dataset?.columns.indexOf(ref.field) ?? -1
  if (!dataset || index < 0) return []
  return dataset.rows.flatMap((row) => {
    const value = row[index]
    return typeof value === 'number' && Number.isFinite(value) ? [value] : []
  })
}

function reduce(values: number[], reducer: VisualizationReferenceReducer): number {
  switch (reducer) {
    case 'first': return values[0]!
    case 'last': return values.at(-1)!
    case 'minimum': return Math.min(...values)
    case 'maximum': return Math.max(...values)
    case 'mean': return values.reduce((sum, value) => sum + value, 0) / values.length
    case 'median': {
      const sorted = [...values].sort((left, right) => left - right)
      const middle = Math.floor(sorted.length / 2)
      return sorted.length % 2 === 0 ? (sorted[middle - 1]! + sorted[middle]!) / 2 : sorted[middle]!
    }
  }
}

function trendPoints(envelope: VisualizationEnvelope, category: VisualizationFieldRef, value: VisualizationFieldRef): KPITrendPoint[] {
  if (envelope.dataState.kind !== 'inline' || category.dataset !== value.dataset) return []
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === category.dataset)
  const categoryIndex = dataset?.columns.indexOf(category.field) ?? -1
  const valueIndex = dataset?.columns.indexOf(value.field) ?? -1
  if (!dataset || categoryIndex < 0 || valueIndex < 0) return []
  return dataset.rows.flatMap((row) => {
    const numeric = row[valueIndex]
    if (typeof numeric !== 'number' || !Number.isFinite(numeric)) return []
    const rawLabel = row[categoryIndex]
    return [{ label: rawLabel === null || rawLabel === undefined ? '—' : String(rawLabel), value: numeric }]
  })
}

function changeStatusFor(delta: number, direction: 'increase' | 'decrease' | 'neutral'): KPIChangeStatus {
  if (delta === 0 || direction === 'neutral') return 'neutral'
  const favorable = direction === 'increase' ? delta > 0 : delta < 0
  return favorable ? 'favorable' : 'unfavorable'
}

function formatField(envelope: VisualizationEnvelope, ref: VisualizationFieldRef, value: number | undefined, context: RendererContext): string {
  const field = envelope.spec.datasets.find((dataset) => dataset.id === ref.dataset)?.fields.find((candidate) => candidate.id === ref.field)
  if (value === undefined) return '—'
  return field?.format ? formatValue(context.locale, field.format, value) : String(value)
}

function fieldFormat(envelope: VisualizationEnvelope, ref: VisualizationFieldRef): VisualizationFormat | undefined {
  return envelope.spec.datasets.find((dataset) => dataset.id === ref.dataset)?.fields.find((candidate) => candidate.id === ref.field)?.format
}

function formatDisplayField(
  envelope: VisualizationEnvelope,
  ref: VisualizationFieldRef,
  value: number | undefined,
  context: RendererContext,
  displayUnit: ResolvedDisplayUnit,
): string {
  if (value === undefined) return '—'
  return formatDisplayValue(context.locale, fieldFormat(envelope, ref) ?? { kind: 'number' }, value, displayUnit)
}

function formatDelta(
  envelope: VisualizationEnvelope,
  ref: VisualizationFieldRef,
  delta: number,
  mode: 'absolute' | 'relative',
  context: RendererContext,
  displayUnit: ResolvedDisplayUnit,
): string {
  if (mode === 'relative') {
    const rounded = Math.round(Math.abs(delta) * 1000) / 10
    return `${delta > 0 ? '+' : delta < 0 ? '−' : ''}${rounded}%`
  }
  const formatted = formatDisplayField(envelope, ref, Math.abs(delta), context, displayUnit)
  return `${delta > 0 ? '+' : delta < 0 ? '−' : ''}${formatted}`
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value))
}

function roundCoordinate(value: number): number {
  return Math.round(value * 100) / 100
}
