import { expect, test } from 'bun:test'
import { buildEChartsOption } from './echarts-adapters'
import { datumForEChartsEvent } from './echarts-renderer'
import { payloadRowIndexFromData } from './utils'
import type { ChartPayload, ChartTokens } from './types'

const tokens: ChartTokens = {
  text: '#111',
  muted: '#666',
  border: '#ddd',
  grid: '#eee',
  surface: '#fff',
  fill: '#acf',
  dimmed: '#bbb',
  palette: ['#06c'],
}

for (const type of ['graph', 'sankey'] as const) {
  test(`${type} renderer only resolves source rows for edges`, () => {
    const payload: ChartPayload = {
      id: `${type}_visual`,
      type,
      shape: 'graph',
      data: [
        { source: 'created', target: 'delivered', value: 7 },
        { source: 'created', target: 'canceled', value: 2 },
      ],
    }
    const option = buildEChartsOption(payload, tokens) as Record<string, unknown>
    const series = (option.series as Array<Record<string, unknown>>)[0]
    const node = (series.data as Array<Record<string, unknown>>)[0]
    const edge = (series.links as Array<Record<string, unknown>>)[1]

    expect(payloadRowIndexFromData(node)).toBeUndefined()
    expect(datumForEChartsEvent(payload, { data: node })).toBeUndefined()
    expect(payloadRowIndexFromData(edge)).toBe(1)
    expect(datumForEChartsEvent(payload, { data: edge })).toEqual({
      datum: payload.data?.[1],
      index: 1,
    })
  })
}
