import type { EChartsOption } from 'echarts'
import * as echarts from 'echarts'
import { registerChartRenderer } from './registry'
import type { ChartPayload, ChartRendererContext, ChartTokens } from './types'
import { deepMerge, normalizeShape } from './utils'
import { buildEChartsOption } from './echarts-adapters'
import { brazilStatesGeoJSON } from './maps'

echarts.registerMap('brazil_states', brazilStatesGeoJSON as never)

registerChartRenderer('echarts', {
  mount(container: HTMLElement, context: ChartRendererContext) {
    const instance = echarts.init(container, null, { renderer: 'canvas' })
    let currentPayload: ChartPayload = {}

    instance.on('click', (event) => {
      const label = selectionValueForEvent(currentPayload, event)
      if (label) context.selectLabel(label)
    })

    return {
      update(payload: ChartPayload, tokens: ChartTokens): void {
        currentPayload = payload
        instance.setOption(buildOption(payload, tokens), true)
        instance.resize()
      },
      resize(): void {
        instance.resize()
      },
      clear(): void {
        instance.clear()
      },
      dispose(): void {
        instance.dispose()
      },
    }
  },
})

function buildOption(payload: ChartPayload, tokens: ChartTokens): EChartsOption {
  const generated = buildEChartsOption(payload, tokens)
  const override = payload.rendererOptions?.echarts ?? {}
  return deepMerge(generated, override) as EChartsOption
}

function selectionValueForEvent(payload: ChartPayload, event: echarts.ECElementEvent): string {
  const shape = normalizeShape(payload.shape, payload.type, Boolean(payload.series?.length))
  const data = (event.data ?? {}) as Record<string, unknown>
  if (shape === 'matrix') return String(data.name || event.name || '')
  if (shape === 'geo') return String(data.name || event.name || '')
  if (shape === 'graph') return String(event.name || data.source || '')
  return String(event.name || data.name || '')
}
