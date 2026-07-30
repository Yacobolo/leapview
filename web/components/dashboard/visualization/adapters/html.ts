import type { VisualizationEnvelope, VisualizationFieldRef } from '../../../../generated/visualization'
import { defaultRendererContext, type RendererAdapter, type RendererContext, type RendererHandle } from '../host-controller'
import { conditionalIconGlyph, conditionalStyleColor, contrastTextColor, resolveConditionalFormat } from '../conditional-format'
import { formatValue } from '../format'
import { resolveVisualizationMetadata } from '../metadata'

export const adapter: RendererAdapter = {
  mount(container, envelope, context) { return new HTMLHandle(container, envelope, context) },
}

class HTMLHandle implements RendererHandle {
  constructor(private readonly container: HTMLElement, envelope: VisualizationEnvelope, context: RendererContext) { this.update(envelope, 0, context) }
  update(envelope: VisualizationEnvelope, _change: number, context: RendererContext): void {
    this.container.replaceChildren()
    const article = document.createElement('article')
    article.className = 'lv-kpi-card'
    const conditional = kpiConditionalPresentation(envelope, context)
    const metadata = resolveVisualizationMetadata(envelope)
    article.setAttribute('aria-label', [
      metadata.title,
      metadata.summary ?? metadata.description,
      conditional.iconLabel ? `Status: ${conditional.iconLabel}.` : '',
    ].filter(Boolean).join('. '))
    if (conditional.background) article.style.backgroundColor = conditional.background
    if (conditional.foreground) article.style.color = conditional.foreground
    if (envelope.spec.kind === 'kpi') article.dataset.tone = envelope.spec.presentation.tone
    const label = document.createElement('div')
    label.className = 'lv-visualization-label'
    label.textContent = metadata.title
    const subtitle = metadata.subtitle ? document.createElement('small') : undefined
    if (subtitle) {
      subtitle.className = 'lv-visualization-note'
      subtitle.textContent = metadata.subtitle!
    }
    const value = document.createElement('strong')
    value.className = 'lv-visualization-kpi'
    if (conditional.valueColor) value.style.color = conditional.valueColor
    value.textContent = [conditional.icon, kpiText(envelope, context)].filter(Boolean).join(' ')
    article.append(label)
    if (subtitle) article.append(subtitle)
    article.append(value)
    if (envelope.spec.kind === 'kpi' && envelope.spec.presentation.note) {
      const note = document.createElement('small'); note.className = 'lv-visualization-note'; note.textContent = envelope.spec.presentation.note; article.append(note)
    }
    this.container.append(article)
  }
  resize(): void {}
  async snapshot(): Promise<Blob> { return new Blob([this.container.textContent ?? ''], { type: 'text/plain' }) }
  dispose(): void { this.container.replaceChildren() }
}

export function kpiConditionalPresentation(envelope: VisualizationEnvelope, context: RendererContext): {
  background?: string
  foreground?: string
  valueColor?: string
  icon?: string
  iconLabel?: string
} {
  const spec = envelope.spec
  if (spec.kind !== 'kpi' || envelope.dataState.kind !== 'inline') return {}
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === spec.value.dataset)
  const row = dataset?.rows[0]
  if (!dataset || !row) return {}
  const resolveTarget = (target: 'visual_background' | 'kpi_value') => {
    const format = spec.conditionalFormatting?.find((candidate) => candidate.target === target)
    return format ? resolveConditionalFormat(format, dataset.columns, row) : undefined
  }
  const backgroundResult = resolveTarget('visual_background')
  const valueResult = resolveTarget('kpi_value')
  const background = backgroundResult ? conditionalStyleColor(backgroundResult.style, (intent) => intentColor(intent, context)) : undefined
  const authoredValueColor = valueResult ? conditionalStyleColor(valueResult.style, (intent) => intentColor(intent, context)) : undefined
  const foreground = background
    ? contrastTextColor(background, [context.colors.foreground, context.colors.surface, '#000000', '#ffffff'])
    : undefined
  const valueColor = background && authoredValueColor
    ? contrastTextColor(background, [authoredValueColor, foreground!, context.colors.surface, '#000000', '#ffffff'])
    : authoredValueColor
  const icon = valueResult?.style.icon ?? backgroundResult?.style.icon
  return {
    ...(background ? { background } : {}),
    ...(foreground ? { foreground } : {}),
    ...(valueColor ? { valueColor } : {}),
    ...(icon ? { icon: conditionalIconGlyph(icon), iconLabel: iconAccessibleLabel(icon) } : {}),
  }
}

export function kpiText(envelope: VisualizationEnvelope, context: RendererContext = defaultRendererContext): string {
  const spec = envelope.spec
  if (spec.kind !== 'kpi') return '—'
  const value = scalar(envelope, spec.value)
  const field = spec.datasets.find((dataset) => dataset.id === spec.value.dataset)?.fields.find((candidate) => candidate.id === spec.value.field)
  if (field?.format) return formatValue(context.locale, field.format, value)
  return value === null || value === undefined ? '—' : String(value)
}

function intentColor(intent: string, context: RendererContext): string {
  switch (intent) {
    case 'accent': return context.colors.accent
    case 'neutral': return context.colors.muted
    case 'ink': return context.colors.foreground
    case 'success': return context.colors.success
    case 'warning': return context.colors.attention
    case 'danger': return context.colors.danger
  }
  if (intent.startsWith('data_')) return context.colors.data[Number(intent.slice(5)) - 1] ?? context.colors.accent
  return context.colors.foreground
}

function iconAccessibleLabel(icon: string): string {
  switch (icon) {
    case 'arrow_up': return 'increasing'
    case 'arrow_down': return 'decreasing'
    case 'triangle_up': return 'higher'
    case 'triangle_down': return 'lower'
    case 'warning': return 'warning'
    default: return icon.replaceAll('_', ' ')
  }
}

function scalar(envelope: VisualizationEnvelope, ref: VisualizationFieldRef): unknown {
  if (envelope.dataState.kind !== 'inline') return undefined
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === ref.dataset)
  const index = dataset?.columns.indexOf(ref.field) ?? -1
  return index >= 0 ? dataset?.rows[0]?.[index] : undefined
}
