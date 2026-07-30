import type { VisualizationEnvelope } from '../../../../../generated/visualization'
import type { RendererContext } from '../../host-controller'
import { axis, field, fieldLabel, inlineDataset, labelFormatter, legend, selectedDatasetSource, toneColor, type EChartsTranslation } from './common'

type CartesianSpec = Extract<VisualizationEnvelope['spec'], { kind: 'cartesian' }>
type ReferenceValue = NonNullable<CartesianSpec['referenceLines']>[number]['value']

export function cartesianOption(envelope: VisualizationEnvelope, context: RendererContext): EChartsTranslation {
  return applyDecisionContext(envelope, context, cartesianBaseOption(envelope, context))
}

function cartesianBaseOption(envelope: VisualizationEnvelope, context: RendererContext): EChartsTranslation {
  const spec = envelope.spec as CartesianSpec
  const horizontal = spec.presentation.orientation === 'horizontal' || spec.mark === 'bar'
  const xType = axisType(envelope, spec.x, horizontal ? 'value' : 'category')
  const xAxis = axis(envelope, horizontal ? spec.y[0]! : spec.x, xType, context)
  const yAxis = axis(envelope, horizontal ? spec.x : spec.y[0]!, horizontal ? 'category' : 'value', context)
  const axes = { grid: { left: 12, right: 16, top: 16, bottom: spec.presentation.dataZoom ? 54 : 16, containLabel: true }, xAxis, yAxis }
  const dataZoom = spec.presentation.dataZoom ? [{ type: 'inside' }, { type: 'slider' }] : undefined
  if (spec.mark === 'histogram') {
    const value = spec.y.find((item) => item.field === 'value') ?? spec.y.at(-1)
    return { ...axes, dataZoom, series: [{ id: seriesID(value?.dataset, value?.field), type: 'bar', encode: { x: spec.x.field, y: value?.field }, label: chartLabel(envelope, value, spec, context) }] }
  }
  if (spec.mark === 'waterfall') {
    const start = spec.y.find((item) => item.field === 'start')
    const value = spec.y.find((item) => item.field === 'value') ?? spec.y[0]
    return {
      ...axes, dataZoom,
      series: [
        { id: 'series:waterfall:offset', type: 'bar', stack: 'waterfall', silent: true, itemStyle: { color: 'transparent' }, encode: { x: spec.x.field, y: start?.field } },
        { id: seriesID(value?.dataset, value?.field), type: 'bar', stack: 'waterfall', encode: { x: spec.x.field, y: value?.field }, label: chartLabel(envelope, value, spec, context) },
      ],
    }
  }
  if (spec.mark === 'candlestick' || spec.mark === 'boxplot') {
    return {
      ...axes, dataZoom, legend: legend(spec.presentation.legend, context),
      series: [{ id: `series:primary:${spec.mark}`, type: spec.mark, name: spec.title, encode: { x: spec.x.field, y: spec.y.map((item) => item.field) } }],
    }
  }
  if (spec.mark === 'heatmap' && spec.y.length >= 2) {
    return {
      xAxis: axis(envelope, spec.x, 'category', context), yAxis: axis(envelope, spec.y[0]!, 'category', context),
      visualMap: { min: 0, calculable: true, orient: 'horizontal', left: 'center', bottom: 0, textStyle: { color: context.colors.muted } },
      series: [{ id: 'series:primary:heatmap', type: 'heatmap', encode: { x: spec.x.field, y: spec.y[0]?.field, value: spec.y[1]?.field }, label: chartLabel(envelope, spec.y[1], spec, context) }],
    }
  }
  const split = splitCartesianSeries(envelope, context)
  if (split) {
    const secondary = split.series.some((item) => item.yAxisIndex === 1)
    return {
      dataset: split.datasets, legend: legend(spec.presentation.legend, context), xAxis: axis(envelope, spec.x, axisType(envelope, spec.x, 'category'), context),
      yAxis: secondary ? [axis(envelope, spec.y[0]!, 'value', context), axis(envelope, spec.y[0]!, 'value', context)] : axis(envelope, spec.y[0]!, 'value', context),
      dataZoom, series: [...split.series, ...interactionHitSeries(envelope, spec, split.series)],
    }
  }
  const series = spec.y.map((value) => ({
    id: seriesID(value.dataset, value.field), type: cartesianSeriesType(spec.mark), name: fieldLabel(envelope, value),
    encode: horizontal ? { x: value.field, y: spec.x.field } : { x: spec.x.field, y: value.field },
    smooth: spec.presentation.smooth, symbol: spec.presentation.showSymbols ? undefined : 'none', symbolSize: spec.presentation.symbolSize,
    stack: spec.presentation.stacked ? 'total' : undefined, areaStyle: spec.presentation.area || spec.mark === 'area' ? {} : undefined,
    step: spec.presentation.step ? 'middle' : false, label: chartLabel(envelope, value, spec, context),
  }))
  return { ...axes, legend: legend(spec.presentation.legend, context), dataZoom, series: [...series, ...interactionHitSeries(envelope, spec, series)] }
}

function applyDecisionContext(envelope: VisualizationEnvelope, context: RendererContext, option: EChartsTranslation): EChartsTranslation {
  const spec = envelope.spec
  if (spec.kind !== 'cartesian') return option
  const accessibilityDetails = [
    ...(spec.referenceLines ?? []).map((line) => line.label ? `Reference line: ${line.label}.` : ''),
    ...(spec.referenceBands ?? []).map((band) => band.label ? `Reference band: ${band.label}.` : ''),
    ...(spec.eventAnnotations ?? []).map((annotation) => `Event: ${annotation.label}${annotation.description ? ` — ${annotation.description}` : ''}.`),
  ].filter(Boolean)
  if (accessibilityDetails.length > 0) {
    const authoredDescription = spec.accessibility.description.trim()
    const description = /[.!?]$/.test(authoredDescription) ? authoredDescription : `${authoredDescription}.`
    option.aria = { enabled: true, description: [description, ...accessibilityDetails].join(' ') }
  }
  for (const authored of spec.axes ?? []) {
    const horizontal = spec.presentation.orientation === 'horizontal' || spec.mark === 'bar'
    const physical = authored.id === 'x'
      ? horizontal ? 'yAxis' : 'xAxis'
      : horizontal ? 'xAxis' : 'yAxis'
    const index = authored.id === 'secondary_y' ? 1 : 0
    const target = axisAt(option, physical, index)
    if (!target) continue
    const title = [authored.title, authored.unit ? `(${authored.unit})` : ''].filter(Boolean).join(' ')
    if (title) target.name = title
    if (authored.scale === 'log') target.type = 'log'
    else if (authored.scale === 'linear') target.type = 'value'
    if (authored.minimum !== undefined) target.min = authored.minimum
    if (authored.maximum !== undefined) target.max = authored.maximum
    if (authored.zero === 'include') target.scale = false
    else if (authored.zero === 'exclude') target.scale = true
    applyTickDensity(target, authored.tickDensity)
  }

  const coordinate = (axisID: 'x' | 'primary_y' | 'secondary_y') => {
    const horizontal = spec.presentation.orientation === 'horizontal' || spec.mark === 'bar'
    if (axisID === 'x') return horizontal ? 'yAxis' : 'xAxis'
    return horizontal ? 'xAxis' : 'yAxis'
  }
  const markLineData = [
    ...(spec.referenceLines ?? []).flatMap((line) => {
      const value = resolveReferenceValue(envelope, line.value)
      if (value === undefined) return []
      return [{
        id: `reference-line:${line.id}`, name: line.label ?? '', [coordinate(line.axis)]: value,
        lineStyle: { color: toneColor(line.tone, context) },
      }]
    }),
    ...(spec.eventAnnotations ?? []).flatMap((annotation) => {
      const value = resolveReferenceValue(envelope, annotation.value)
      if (value === undefined) return []
      return [{
        id: `event-annotation:${annotation.id}`, name: annotation.label, [coordinate(annotation.axis)]: value,
        lineStyle: { color: toneColor(annotation.tone, context) },
      }]
    }),
  ]
  const markAreaData = (spec.referenceBands ?? []).flatMap((band) => {
    const from = resolveReferenceValue(envelope, band.from)
    const to = resolveReferenceValue(envelope, band.to)
    if (from === undefined || to === undefined) return []
    const key = coordinate(band.axis)
    return [[
      { id: `reference-band:${band.id}`, name: band.label ?? '', [key]: from, itemStyle: { color: toneColor(band.tone, context), opacity: 0.12 } },
      { [key]: to },
    ]]
  })
  if (markLineData.length === 0 && markAreaData.length === 0) return option
  const series = Array.isArray(option.series) ? option.series : []
  const owner = series.find((candidate: EChartsTranslation) => !candidate.silent && !String(candidate.id ?? '').startsWith('series:interaction-hit:'))
  if (!owner) return option
  if (markLineData.length > 0) owner.markLine = { symbol: ['none', 'none'], data: markLineData }
  if (markAreaData.length > 0) owner.markArea = { silent: true, data: markAreaData }
  return option
}

function axisAt(option: EChartsTranslation, key: 'xAxis' | 'yAxis', index: number): EChartsTranslation | undefined {
  const current = option[key]
  if (Array.isArray(current)) return current[index]
  if (index === 0) return current
  if (!current) return undefined
  const secondary = structuredClone(current)
  option[key] = [current, secondary]
  return secondary
}

function applyTickDensity(axisOption: EChartsTranslation, density: 'automatic' | 'sparse' | 'normal' | 'dense'): void {
  if (density === 'automatic') return
  if (axisOption.type === 'category' || axisOption.type === 'time') {
    axisOption.axisLabel = { ...axisOption.axisLabel, interval: density === 'sparse' ? 2 : density === 'dense' ? 0 : 'auto' }
    return
  }
  axisOption.splitNumber = density === 'sparse' ? 3 : density === 'dense' ? 8 : 5
}

function resolveReferenceValue(envelope: VisualizationEnvelope, value: ReferenceValue): string | number | undefined {
  if (value.kind === 'number' || value.kind === 'text') return value.value
  const dataset = inlineDataset(envelope, value.field.dataset)
  const index = dataset?.columns.indexOf(value.field.field) ?? -1
  if (!dataset || index < 0) return undefined
  const values = dataset.rows.map((row) => row[index]).filter((candidate): candidate is string | number => typeof candidate === 'string' || typeof candidate === 'number')
  if (values.length === 0) return undefined
  switch (value.reducer) {
    case 'first': return values[0]
    case 'last': return values.at(-1)
    case 'minimum': return orderedReferenceValue(values, 'minimum')
    case 'maximum': return orderedReferenceValue(values, 'maximum')
    case 'mean': {
      const numbers = values.filter((candidate): candidate is number => typeof candidate === 'number' && Number.isFinite(candidate))
      return numbers.length === values.length ? numbers.reduce((sum, candidate) => sum + candidate, 0) / numbers.length : undefined
    }
    case 'median': {
      const numbers = values.filter((candidate): candidate is number => typeof candidate === 'number' && Number.isFinite(candidate)).sort((left, right) => left - right)
      if (numbers.length !== values.length) return undefined
      const middle = Math.floor(numbers.length / 2)
      return numbers.length % 2 ? numbers[middle] : (numbers[middle - 1]! + numbers[middle]!) / 2
    }
  }
}

function orderedReferenceValue(values: (string | number)[], reducer: 'minimum' | 'maximum'): string | number | undefined {
  if (values.every((value) => typeof value === 'number')) {
    return reducer === 'minimum' ? Math.min(...values as number[]) : Math.max(...values as number[])
  }
  if (values.every((value) => typeof value === 'string')) {
    return [...values as string[]].sort((left, right) => left.localeCompare(right, 'en'))[reducer === 'minimum' ? 0 : values.length - 1]
  }
  return undefined
}

function interactionHitSeries(envelope: VisualizationEnvelope, spec: CartesianSpec, series: EChartsTranslation[]): EChartsTranslation[] {
  if (!spec.interactions.some((interaction) => interaction.kind === 'select')) return []
  return series.flatMap((candidate, index) => {
    if (candidate.type !== 'line') return []
    const yField = typeof candidate.encode?.y === 'string' ? candidate.encode.y : spec.y[index]?.field ?? `value-${index}`
    const identity = candidate.datasetId
      ? `${spec.x.dataset}:${spec.x.field}:${encodeURIComponent(String(candidate.datasetId))}`
      : `${spec.x.dataset}:${spec.x.field}:${yField}`
    return [{
      id: `series:interaction-hit:${identity}`,
      type: 'scatter',
      ...(candidate.datasetId ? { datasetId: candidate.datasetId } : {}),
      encode: candidate.encode,
      ...(candidate.xAxisIndex !== undefined ? { xAxisIndex: candidate.xAxisIndex } : {}),
      ...(candidate.yAxisIndex !== undefined ? { yAxisIndex: candidate.yAxisIndex } : {}),
      symbolSize: Math.max(18, spec.presentation.symbolSize ?? 0),
      itemStyle: { color: 'rgba(0,0,0,0.001)' },
      emphasis: { disabled: true },
      tooltip: { show: false },
      silent: false,
      z: 10,
    }]
  })
}

function chartLabel(envelope: VisualizationEnvelope, value: CartesianSpec['y'][number] | undefined, spec: CartesianSpec, context: RendererContext) {
  const authored = spec.presentation.labelPosition
  const horizontal = spec.presentation.orientation === 'horizontal' || spec.mark === 'bar'
  const position = authored === 'automatic' ? undefined : authored === 'outside' ? horizontal ? 'right' : 'top' : authored
  return { show: spec.presentation.showLabels, position, formatter: labelFormatter(envelope, value, context) }
}

function splitCartesianSeries(envelope: VisualizationEnvelope, context: RendererContext): { datasets: EChartsTranslation[]; series: EChartsTranslation[] } | undefined {
  const spec = envelope.spec
  if (spec.kind !== 'cartesian' || !spec.series || spec.y.length !== 1 || envelope.dataState.kind !== 'inline') return undefined
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === spec.series?.dataset)
  const seriesIndex = dataset?.columns.indexOf(spec.series.field) ?? -1
  if (!dataset || seriesIndex < 0) return undefined
  const available = [...new Set(dataset.rows.map((row) => row[seriesIndex]).filter((value): value is string | number | boolean => typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean'))]
  const configured = new Map((spec.presentation.comboSeries ?? []).map((item) => [String(item.seriesValue), item]))
  const configuredOrder = (spec.presentation.comboSeries ?? []).map((item) => String(item.seriesValue))
  const values = [
    ...configuredOrder.filter((value) => available.some((candidate) => String(candidate) === value)),
    ...available.filter((value) => !configured.has(String(value))).sort((left, right) => String(left).localeCompare(String(right), 'en')),
  ]
  const datasets: EChartsTranslation[] = [{ id: `dataset:${dataset.id}`, source: selectedDatasetSource(envelope, dataset) }]
  const series = values.map((value) => {
    const token = encodeURIComponent(String(value))
    const datasetID = `dataset:series:${spec.series?.field}:${token}`
    datasets.push({ id: datasetID, fromDatasetId: `dataset:${dataset.id}`, transform: { type: 'filter', config: { dimension: spec.series?.field, '=': value } } })
    const combo = configured.get(String(value))
    const mark = combo?.mark ?? (spec.mark === 'combo' ? 'line' : spec.mark)
    return {
      id: `series:${spec.series?.dataset}:${spec.series?.field}:${token}`, datasetId: datasetID, name: String(value), type: cartesianSeriesType(mark), yAxisIndex: combo?.axis === 'secondary' ? 1 : 0,
      encode: { x: spec.x.field, y: spec.y[0]?.field }, smooth: spec.presentation.smooth, symbol: spec.presentation.showSymbols ? undefined : 'none',
      stack: spec.presentation.stacked ? 'total' : undefined, areaStyle: spec.presentation.area || mark === 'area' ? {} : undefined,
      step: spec.presentation.step ? 'middle' : false, label: chartLabel(envelope, spec.y[0], spec, context),
    }
  })
  return { datasets, series }
}

function axisType(envelope: VisualizationEnvelope, ref: CartesianSpec['x'], fallback: 'category' | 'value'): 'category' | 'value' | 'time' {
  const dataType = field(envelope, ref)?.dataType
  return dataType === 'temporal' || dataType === 'date' ? 'time' : fallback
}

function seriesID(dataset = 'primary', value = 'value'): string { return `series:${dataset}:${value}` }

function cartesianSeriesType(mark: CartesianSpec['mark']): string {
  switch (mark) {
    case 'bar': case 'column': case 'waterfall': case 'histogram': return 'bar'
    case 'scatter': return 'scatter'
    case 'candlestick': return 'candlestick'
    case 'boxplot': return 'boxplot'
    default: return 'line'
  }
}
