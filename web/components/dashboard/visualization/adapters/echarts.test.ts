import { expect, test } from 'bun:test'

import type { VisualizationEnvelope } from '../../../../generated/visualization'
import { echartsOption, interactionCommandForRow } from './echarts'

test('ECharts translation uses dataset and encode without native option passthrough', () => {
  const envelope = {
    schemaVersion: 1, visualID: 'revenue', rendererID: 'echarts', specRevision: 'sha256:test', dataRevision: 1,
    spec: {
      kind: 'cartesian', title: 'Revenue', mark: 'line',
      datasets: [{ id: 'primary', fields: [
        { id: 'month', role: 'dimension', dataType: 'string', nullable: false, label: 'Month' },
        { id: 'revenue', role: 'measure', dataType: 'decimal', nullable: false, label: 'Revenue' },
      ] }],
      dataBudget: { maxRows: 100, requiredCompleteness: 'complete' }, accessibility: { title: 'Revenue', description: 'Revenue by month' }, interactions: [],
      x: { dataset: 'primary', field: 'month' }, y: [{ dataset: 'primary', field: 'revenue' }],
      presentation: { legend: 'bottom', showLabels: false, smooth: true, stacked: false, showSymbols: true, dataZoom: false, area: false, step: false },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 1, generation: 1, datasets: [
      { id: 'primary', specRevision: 'sha256:test', dataRevision: 1, generation: 1, columns: ['month', 'revenue'], rows: [['Jan', 10]], completeness: 'complete' },
    ] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  const option = echartsOption(envelope) as any
  expect(option.dataset.source).toEqual([['month', 'revenue'], ['Jan', 10]])
  expect(option.series[0].encode).toEqual({ x: 'month', y: 'revenue' })
  expect(JSON.stringify(option)).not.toContain('rendererOptions')
})

test('ECharts interactions translate stable IR field mappings without renderer row keys', () => {
  const envelope = {
    schemaVersion: 1, visualID: 'orders', rendererID: 'echarts', specRevision: 'sha256:test', dataRevision: 7,
    spec: {
      kind: 'cartesian', title: 'Orders', mark: 'bar',
      datasets: [{ id: 'primary', fields: [
        { id: 'status', role: 'identity', dataType: 'string', nullable: false, label: 'Status' },
        { id: 'count', role: 'measure', dataType: 'integer', nullable: false, label: 'Orders' },
      ] }],
      dataBudget: { maxRows: 100, requiredCompleteness: 'complete' }, accessibility: { title: 'Orders', description: 'Orders by status' },
      interactions: [{ id: 'point_selection', kind: 'select', mode: 'multiple', requiresStableIdentity: true, targets: ['details'], mappings: [
        { source: { dataset: 'primary', field: 'status' }, targetFieldID: 'orders.status', targetFactID: 'orders', label: { dataset: 'primary', field: 'status' } },
      ] }],
      x: { dataset: 'primary', field: 'status' }, y: [{ dataset: 'primary', field: 'count' }],
      presentation: { legend: 'bottom', showLabels: false, smooth: false, stacked: false, showSymbols: true, dataZoom: false, area: false, step: false },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 7, generation: 2, datasets: [
      { id: 'primary', specRevision: 'sha256:test', dataRevision: 7, generation: 2, columns: ['status', 'count'], rows: [['delivered', 42]], completeness: 'complete' },
    ] },
    selection: [{ datum: { dataset: 'primary', dataRevision: 7, identity: { status: 'delivered' } }, label: 'Delivered' }], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  expect(interactionCommandForRow(envelope, 'primary', ['delivered', 42])).toEqual({
    sourceKind: 'visual', sourceId: 'orders', interactionKind: 'point_selection', action: 'set', toggle: true,
    mappings: [{ field: 'orders.status', fact: 'orders', value: 'delivered', label: 'delivered' }],
  })
  expect(interactionCommandForRow(envelope, 'primary', [{ forged: true }, 42])).toBeUndefined()
  const option = echartsOption(envelope) as any
  expect(option.dataset.source).toEqual([['status', 'count', '__ld_selected'], ['delivered', 42, true]])
  expect(option.visualMap.dimension).toBe('__ld_selected')
})

test('ECharts translation preserves combo series marks and axes', () => {
  const base = {
    schemaVersion: 1, visualID: 'combo', rendererID: 'echarts', specRevision: 'sha256:test', dataRevision: 1,
    spec: {
      kind: 'cartesian', title: 'Combo', mark: 'combo',
      datasets: [{ id: 'primary', fields: [
        { id: 'month', role: 'dimension', dataType: 'string', nullable: false, label: 'Month' },
        { id: 'series', role: 'dimension', dataType: 'string', nullable: false, label: 'Series' },
        { id: 'value', role: 'measure', dataType: 'decimal', nullable: false, label: 'Value' },
      ] }],
      dataBudget: { maxRows: 100, requiredCompleteness: 'complete' }, accessibility: { title: 'Combo', description: 'Combo' }, interactions: [],
      x: { dataset: 'primary', field: 'month' }, y: [{ dataset: 'primary', field: 'value' }], series: { dataset: 'primary', field: 'series' },
      presentation: { legend: 'bottom', showLabels: false, smooth: false, stacked: false, showSymbols: true, dataZoom: false, area: false, step: false, comboSeries: [
        { seriesValue: 'Revenue', mark: 'line', axis: 'primary' },
        { seriesValue: 'Orders', mark: 'column', axis: 'secondary' },
      ] },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 1, generation: 1, datasets: [
      { id: 'primary', specRevision: 'sha256:test', dataRevision: 1, generation: 1, columns: ['month', 'series', 'value'], rows: [['Jan', 'Revenue', 10], ['Jan', 'Orders', 2]], completeness: 'complete' },
    ] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  const option = echartsOption(base) as any
  expect(option.dataset).toHaveLength(3)
  expect(option.series.map((series: any) => [series.name, series.type, series.yAxisIndex])).toEqual([
    ['Revenue', 'line', 0], ['Orders', 'bar', 1],
  ])
  expect(option.yAxis).toHaveLength(2)
})

test('ECharts translation emits one multi-value financial series', () => {
  const envelope = {
    schemaVersion: 1, visualID: 'ohlc', rendererID: 'echarts', specRevision: 'sha256:test', dataRevision: 1,
    spec: {
      kind: 'cartesian', title: 'OHLC', mark: 'candlestick',
      datasets: [{ id: 'primary', fields: ['label', 'open', 'close', 'low', 'high'].map((id, index) => ({ id, role: index ? 'measure' : 'dimension', dataType: index ? 'decimal' : 'string', nullable: false, label: id })) }],
      dataBudget: { maxRows: 100, requiredCompleteness: 'complete' }, accessibility: { title: 'OHLC', description: 'OHLC' }, interactions: [],
      x: { dataset: 'primary', field: 'label' }, y: ['open', 'close', 'low', 'high'].map((field) => ({ dataset: 'primary', field })),
      presentation: { legend: 'hidden', showLabels: false, smooth: false, stacked: false, showSymbols: false, dataZoom: true, area: false, step: false },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 1, generation: 1, datasets: [
      { id: 'primary', specRevision: 'sha256:test', dataRevision: 1, generation: 1, columns: ['label', 'open', 'close', 'low', 'high'], rows: [['Jan', 1, 2, 0, 3]], completeness: 'complete' },
    ] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  const option = echartsOption(envelope) as any
  expect(option.series).toHaveLength(1)
  expect(option.series[0].encode).toEqual({ x: 'label', y: ['open', 'close', 'low', 'high'] })
})

test('ECharts translation builds radar indicators and aligned series from typed fields', () => {
  const envelope = {
    schemaVersion: 1, visualID: 'quality', rendererID: 'echarts', specRevision: 'sha256:test', dataRevision: 1,
    spec: {
      kind: 'polar', title: 'Quality', mark: 'radar',
      datasets: [{ id: 'primary', fields: [
        { id: 'metric', role: 'dimension', dataType: 'string', nullable: false, label: 'Metric' },
        { id: 'team', role: 'dimension', dataType: 'string', nullable: false, label: 'Team' },
        { id: 'value', role: 'measure', dataType: 'decimal', nullable: false, label: 'Value' },
      ] }],
      dataBudget: { maxRows: 100, requiredCompleteness: 'complete' }, accessibility: { title: 'Quality', description: 'Quality by team' }, interactions: [],
      category: { dataset: 'primary', field: 'metric' }, series: { dataset: 'primary', field: 'team' }, value: { dataset: 'primary', field: 'value' },
      presentation: { legend: 'bottom', showLabels: false, showPointer: false, area: true },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 1, generation: 1, datasets: [{
      id: 'primary', specRevision: 'sha256:test', dataRevision: 1, generation: 1, columns: ['metric', 'team', 'value'],
      rows: [['Speed', 'A', 8], ['Quality', 'A', 9], ['Speed', 'B', 6], ['Quality', 'B', 7]], completeness: 'complete',
    }] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope
  const option = echartsOption(envelope) as any
  expect(option.radar.indicator.map((item: any) => item.name)).toEqual(['Speed', 'Quality'])
  expect(option.series[0].data).toEqual([{ name: 'A', value: [8, 9] }, { name: 'B', value: [6, 7] }])
})
