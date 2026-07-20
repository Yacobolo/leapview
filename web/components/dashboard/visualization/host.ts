import { LitElement, css, html } from 'lit'
import { property, query, state } from 'lit/decorators.js'
import type { VisualizationEnvelope } from '../../../generated/visualization'
import validateGeneratedEnvelope from '../../../generated/visualization/validate'
import { VisualizationController, validateEnvelopeBoundary } from './host-controller'
import { visualizationRegistry } from './registry'

export class VisualizationHost extends LitElement {
  @property({ attribute: false }) envelope?: VisualizationEnvelope
  @query('.renderer') private rendererContainer?: HTMLDivElement
  @state() private error = ''
  @state() private applying = false
  private controller?: VisualizationController
  private resizeObserver?: ResizeObserver
  private applyGeneration = 0

  static styles = css`
    :host, .surface, .renderer { display: block; width: 100%; height: 100%; min-width: 0; min-height: 0; }
    .surface { position: relative; }
    .error { position: absolute; inset: 0; display: grid; place-items: center; color: var(--ld-fg-danger); padding: 1rem; text-align: center; background: var(--ld-bg-panel); }
    .fallback { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
  `

  protected firstUpdated(): void {
    if (!this.rendererContainer) return
    this.controller = new VisualizationController(
      visualizationRegistry,
      this.rendererContainer,
      (value): value is VisualizationEnvelope => validateGeneratedEnvelope(value) && validateEnvelopeBoundary(value),
      (detail) => this.dispatchEvent(new CustomEvent('ld-visualization-observation', { bubbles: true, composed: true, detail })),
    )
    this.resizeObserver = new ResizeObserver(([entry]) => {
      if (!entry) return
      this.controller?.resize(entry.contentRect.width, entry.contentRect.height, window.devicePixelRatio || 1)
    })
    this.resizeObserver.observe(this)
    void this.applyEnvelope()
  }

  protected updated(changed: Map<PropertyKey, unknown>): void {
    if (changed.has('envelope')) void this.applyEnvelope()
  }

  disconnectedCallback(): void {
    this.resizeObserver?.disconnect()
    this.controller?.dispose()
    this.controller = undefined
    super.disconnectedCallback()
  }

  async snapshot(): Promise<Blob> { return this.controller?.snapshot() ?? Promise.reject(new Error('visualization is not mounted')) }

  protected render() {
    const statusError = this.envelope?.status.kind === 'error' ? this.envelope.status.message ?? 'Visualization error' : ''
    const error = this.error || statusError
    return html`<div class="surface">
      <div class="renderer" role="group" aria-label=${this.envelope?.spec.accessibility.title ?? 'Visualization'} aria-describedby="visualization-fallback" aria-busy=${String(this.applying)}></div>
      <div id="visualization-fallback" class="fallback">${this.accessibleFallback()}</div>
      ${error ? html`<div class="error" role="alert">${error}</div>` : null}
    </div>`
  }

  private async applyEnvelope(): Promise<void> {
    if (!this.envelope || !this.controller) return
    const generation = ++this.applyGeneration
    this.applying = true
    try {
      await this.controller.apply(this.envelope)
      if (generation === this.applyGeneration) this.error = ''
    } catch (error) {
      if (generation === this.applyGeneration) this.error = error instanceof Error ? error.message : String(error)
    } finally {
      if (generation === this.applyGeneration) this.applying = false
    }
  }

  private accessibleFallback() {
    const envelope = this.envelope
    if (!envelope) return 'Visualization is loading.'
    const status = envelope.status.message ?? envelope.status.kind.replaceAll('_', ' ')
    const summary = envelope.spec.accessibility.summary ?? envelope.spec.accessibility.description
    return `${envelope.spec.accessibility.title}. ${summary}. Status: ${status}.`
  }
}

if (!customElements.get('ld-visualization-host')) customElements.define('ld-visualization-host', VisualizationHost)

declare global { interface HTMLElementTagNameMap { 'ld-visualization-host': VisualizationHost } }
