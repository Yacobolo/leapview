import type { VisualizationEnvelope } from '../../../../../generated/visualization'
import { inlineDataset } from './common'

type HeatmapRangeDescriptor = Readonly<{
  minimum: number
  maximum: number
  precision: number
}>

export function heatmapRangeDescriptor(envelope: VisualizationEnvelope): HeatmapRangeDescriptor | undefined {
  const spec = envelope.spec
  if (spec.kind !== 'cartesian' || spec.mark !== 'heatmap' || spec.y.length < 2) return undefined
  const ref = spec.y[1]!
  const dataset = inlineDataset(envelope, ref.dataset)
  const index = dataset?.columns.indexOf(ref.field) ?? -1
  const values = index < 0 ? [] : (dataset?.rows ?? []).flatMap((row) => {
    const value = row[index]
    return typeof value === 'number' && Number.isFinite(value) ? [value] : []
  })
  if (values.length === 0) return { minimum: 0, maximum: 1, precision: 0 }

  const precision = Math.min(12, Math.max(0, ...values.map(decimalPrecision)))
  const minimum = Math.min(...values)
  const maximum = Math.max(...values)
  if (minimum !== maximum) return { minimum, maximum, precision }
  if (maximum > 0) return { minimum: 0, maximum, precision }
  if (minimum < 0) return { minimum, maximum: 0, precision }
  return { minimum: 0, maximum: 1, precision }
}

export function normalizeHeatmapRangeSelection(
  envelope: VisualizationEnvelope,
  selected: unknown,
): [number, number] | undefined {
  const descriptor = heatmapRangeDescriptor(envelope)
  if (!descriptor || !Array.isArray(selected) || selected.length !== 2) return undefined
  const lower = selected[0]
  const upper = selected[1]
  if (typeof lower !== 'number' || !Number.isFinite(lower) || typeof upper !== 'number' || !Number.isFinite(upper)) return undefined
  const normalized = [roundToPrecision(lower, descriptor.precision), roundToPrecision(upper, descriptor.precision)] as [number, number]
  return normalized[0] <= normalized[1] ? normalized : [normalized[1], normalized[0]]
}

function decimalPrecision(value: number): number {
  const [coefficient, exponentText] = Math.abs(value).toString().toLowerCase().split('e')
  const fractionDigits = coefficient?.split('.')[1]?.length ?? 0
  const exponent = Number.parseInt(exponentText ?? '0', 10)
  return Math.max(0, fractionDigits - exponent)
}

function roundToPrecision(value: number, precision: number): number {
  return Number(value.toFixed(precision))
}
