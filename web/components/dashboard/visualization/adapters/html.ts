import type { VisualizationEnvelope, VisualizationFieldRef } from '../../../../generated/visualization'
import type { RendererAdapter, RendererHandle } from '../host-controller'
import { formatValue } from '../format'

export const adapter: RendererAdapter = {
  mount(container, envelope) { return new HTMLHandle(container, envelope) },
}

class HTMLHandle implements RendererHandle {
  constructor(private readonly container: HTMLElement, envelope: VisualizationEnvelope) { this.update(envelope) }
  update(envelope: VisualizationEnvelope): void {
    this.container.replaceChildren()
    const article = document.createElement('article')
    article.setAttribute('aria-label', envelope.spec.accessibility.title)
    const label = document.createElement('div')
    label.className = 'ld-visualization-label'
    label.textContent = envelope.spec.title
    const value = document.createElement('strong')
    value.className = 'ld-visualization-kpi'
    value.textContent = kpiText(envelope)
    article.append(label, value)
    if (envelope.spec.kind === 'kpi' && envelope.spec.presentation.note) {
      const note = document.createElement('small'); note.textContent = envelope.spec.presentation.note; article.append(note)
    }
    this.container.append(article)
  }
  resize(): void {}
  async snapshot(): Promise<Blob> { return new Blob([this.container.textContent ?? ''], { type: 'text/plain' }) }
  dispose(): void { this.container.replaceChildren() }
}

export function kpiText(envelope: VisualizationEnvelope): string {
  const spec = envelope.spec
  if (spec.kind !== 'kpi') return '—'
  const value = scalar(envelope, spec.value)
  const field = spec.datasets.find((dataset) => dataset.id === spec.value.dataset)?.fields.find((candidate) => candidate.id === spec.value.field)
  if (field?.format) return formatValue('en-US', field.format, value)
  return value === null || value === undefined ? '—' : String(value)
}

function scalar(envelope: VisualizationEnvelope, ref: VisualizationFieldRef): unknown {
  if (envelope.dataState.kind !== 'inline') return undefined
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === ref.dataset)
  const index = dataset?.columns.indexOf(ref.field) ?? -1
  return index >= 0 ? dataset?.rows[0]?.[index] : undefined
}
