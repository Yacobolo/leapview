import type { VisualizationEnvelope } from '../../../../generated/visualization'
import type { ECharts, EChartsOption } from 'echarts'
import { Change, defaultRendererContext, normalizeRendererLocale, type RendererAdapter, type RendererContext, type RendererHandle } from '../host-controller'
import { interactionCommandForRow } from '../interaction-command'
import { baseOption } from './echarts/common'
import { cartesianOption } from './echarts/cartesian'
import { hierarchyOption } from './echarts/hierarchy'
import { polarOption } from './echarts/polar'
import { proportionalOption } from './echarts/proportional'

export { interactionCommandForRow, normalizeRendererLocale }

export function echartsOption(envelope: VisualizationEnvelope, context: RendererContext = defaultRendererContext): EChartsOption {
  const base = baseOption(envelope, context)
  let translated: Record<string, any>
  switch (envelope.spec.kind) {
    case 'cartesian': translated = cartesianOption(envelope, context); break
    case 'proportional': translated = proportionalOption(envelope, context); break
    case 'hierarchy': translated = hierarchyOption(envelope, context); break
    case 'polar': translated = polarOption(envelope, context); break
    default: throw new Error(`ECharts cannot render visualization kind ${JSON.stringify(envelope.spec.kind)}`)
  }
  const option = { ...base, ...translated } as Record<string, any>
  if (base.graphic && translated.graphic) option.graphic = [...base.graphic, ...translated.graphic]
  return option as EChartsOption
}

export const adapter: RendererAdapter = {
  async mount(container, envelope, context) {
    const echarts = await import('echarts')
    const frame = createEChartsRendererFrame(container)
    const chart = echarts.init(frame, undefined, { renderer: 'canvas', devicePixelRatio: context.devicePixelRatio })
    const handle = new EChartsHandle(container, frame, chart)
    try {
      handle.mount(envelope, context)
      return handle
    } catch (error) {
      handle.dispose()
      throw error
    }
  },
}

export function createEChartsRendererFrame(container: HTMLElement, createFrame: () => HTMLElement = () => document.createElement('div')): HTMLElement {
  const frame = createFrame()
  frame.style.cssText = 'display:block;width:100%;height:100%;min-width:0;min-height:0;overflow:hidden'
  container.replaceChildren(frame)
  return frame
}

export function removeEChartsRendererFrame(container: ParentNode, frame: HTMLElement): void {
  if (frame.parentNode === container) frame.remove()
}

class EChartsHandle implements RendererHandle {
  private envelope?: VisualizationEnvelope
  private disposed = false
  private readiness: Promise<void> = Promise.resolve()
  private readinessAbort?: AbortController

  constructor(private readonly container: HTMLElement, private readonly frame: HTMLElement, private readonly chart: ECharts) {
    this.chart.on('click', this.handleClick)
  }

  mount(envelope: VisualizationEnvelope, context: RendererContext): void {
    this.envelope = envelope
    this.readinessAbort?.abort()
    this.readinessAbort = new AbortController()
    this.readiness = waitForEChartsFrame(this.chart, 5_000, this.readinessAbort.signal)
    this.chart.setOption(echartsOption(envelope, context), { notMerge: true, lazyUpdate: false })
  }

  whenReady(): Promise<void> { return this.readiness }

  update(envelope: VisualizationEnvelope, change: Change, context: RendererContext): void {
    if (this.disposed) return
    this.envelope = envelope
    const option = echartsOption(envelope, context)
    const plan = echartsUpdatePlan(change, option)
    this.chart.setOption(plan.option, plan.settings)
  }

  resize(width: number, height: number): void { this.chart.resize({ width, height, silent: true }) }

  async snapshot(): Promise<Blob> {
    const response = await fetch(this.chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: 'transparent' }))
    return response.blob()
  }

  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.readinessAbort?.abort()
    this.readinessAbort = undefined
    this.chart.off('click', this.handleClick)
    this.chart.dispose()
    removeEChartsRendererFrame(this.container, this.frame)
  }

  private readonly handleClick = (params: unknown) => {
    const envelope = this.envelope
    if (!envelope) return
    const event = params as { value?: unknown; data?: { __lv_dataset?: unknown; __lv_row_index?: unknown } }
    let datasetID: string | undefined
    let row: unknown[] | undefined
    if (Array.isArray(event.value)) {
      datasetID = envelope.spec.interactions.find((candidate) => candidate.kind === 'select')?.mappings[0]?.source.dataset
      row = event.value
    } else if (typeof event.data?.__lv_dataset === 'string' && Number.isInteger(event.data.__lv_row_index)) {
      datasetID = event.data.__lv_dataset
      if (envelope.dataState.kind === 'inline') {
        row = envelope.dataState.datasets.find((candidate) => candidate.id === datasetID)?.rows[event.data.__lv_row_index as number]
      }
    }
    if (!datasetID || !row) return
    const command = interactionCommandForRow(envelope, datasetID, row)
    if (!command) return
    this.container.dispatchEvent(new CustomEvent('lv-interaction-select', { bubbles: true, composed: true, detail: command }))
  }
}

export type EChartsUpdatePlan = Readonly<{
  option: Record<string, any>
  settings: { notMerge: boolean; lazyUpdate: boolean; replaceMerge?: string[] }
}>

export function echartsUpdatePlan(change: Change, option: EChartsOption): EChartsUpdatePlan {
  if ((change & Change.Spec) !== 0) {
    return { option: option as Record<string, any>, settings: { notMerge: true, lazyUpdate: false } }
  }
  const source = option as Record<string, any>
  const patch: Record<string, any> = {}
  const replaceMerge: string[] = []
  if ((change & Change.Data) !== 0) {
    patch.dataset = source.dataset
    patch.series = source.series
    patch.visualMap = source.visualMap ?? []
    replaceMerge.push('dataset', 'series', 'visualMap')
  } else if ((change & Change.Selection) !== 0) {
    patch.dataset = source.dataset
    patch.visualMap = source.visualMap ?? []
    replaceMerge.push('dataset', 'visualMap')
  }
  if ((change & Change.Status) !== 0) {
    patch.title = source.title ?? []
    patch.graphic = source.graphic ?? []
    replaceMerge.push('title', 'graphic')
  }
  if ((change & Change.Context) !== 0) Object.assign(patch, echartsContextPatch(source))
  return {
    option: patch,
    settings: { notMerge: false, lazyUpdate: (change & Change.Data) === 0, ...(replaceMerge.length ? { replaceMerge } : {}) },
  }
}

function echartsContextPatch(option: Record<string, any>): Record<string, any> {
  const patch: Record<string, any> = {}
  for (const key of ['backgroundColor', 'color', 'textStyle', 'tooltip', 'legend', 'xAxis', 'yAxis', 'radar', 'graphic', 'title']) {
    if (option[key] !== undefined) patch[key] = option[key]
  }
  if (Array.isArray(option.series)) {
    patch.series = option.series.map((raw: Record<string, any>) => {
      const { data: _data, links: _links, encode: _encode, datasetId: _datasetID, ...series } = raw
      return series
    })
  }
  return patch
}

type EChartsFrameChart = Pick<ECharts, 'on' | 'off' | 'getWidth' | 'getHeight'>

export class EChartsReadinessError extends Error {
  constructor(readonly reason: 'timeout' | 'invalid_layout', readonly width: number, readonly height: number) {
    super(reason === 'invalid_layout'
      ? `ECharts cannot render its first frame with invalid layout ${width}x${height}`
      : 'ECharts did not complete its first frame')
    this.name = 'EChartsReadinessError'
  }
}

export function waitForEChartsFrame(chart: EChartsFrameChart, timeoutMs = 5_000, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout> | undefined
    let settled = false
    const cleanup = () => {
      if (timer !== undefined) clearTimeout(timer)
      chart.off('rendered', rendered)
      signal?.removeEventListener('abort', aborted)
    }
    const complete = (action: () => void) => {
      if (settled) return
      settled = true
      cleanup()
      action()
    }
    const rendered = () => {
      const width = chart.getWidth()
      const height = chart.getHeight()
      if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return
      complete(resolve)
    }
    const aborted = () => { complete(resolve) }
    chart.on('rendered', rendered)
    if (signal?.aborted) {
      aborted()
      return
    }
    signal?.addEventListener('abort', aborted, { once: true })
    timer = setTimeout(() => {
      const width = chart.getWidth()
      const height = chart.getHeight()
      complete(() => reject(new EChartsReadinessError(width > 0 && height > 0 ? 'timeout' : 'invalid_layout', width, height)))
    }, timeoutMs)
  })
}
