import { LitElement, css, html, svg as svgTemplate } from 'lit'
import { property, state } from 'lit/decorators.js'

type VisualElement = HTMLElement & {
  dataset: DOMStringMap
}

type ZoomMode = 'fit-page' | 'custom'

type ZoomCommand = {
  mode?: ZoomMode
  scale?: number
}

class ReportCanvas extends LitElement {
  @property({ type: Number }) width = 1366
  @property({ type: Number }) height = 768
  @state() private scale = 1
  @state() private zoomMode: ZoomMode = storedZoomMode()
  private customScale = storedCustomScale()

  private resizeObserver?: ResizeObserver

  static styles = css`
    :host {
      display: block;
      width: 100%;
      max-width: 100%;
      min-width: 0;
      box-sizing: border-box;
    }

    .surface {
      width: 100%;
      min-width: 0;
      background: var(--report-canvas-bg, var(--bgColor-inset));
    }

    .viewport {
      position: relative;
      width: 100%;
      min-width: 0;
      overflow: auto hidden;
      padding: 0;
    }

    .frame {
      position: relative;
      box-sizing: border-box;
      width: calc(var(--report-canvas-width) * 1px);
      height: calc(var(--report-canvas-height) * 1px);
      transform: scale(var(--report-canvas-scale));
      transform-origin: top left;
      background: var(--report-page-bg, transparent);
    }

    .sizer {
      position: relative;
      width: calc(var(--report-canvas-width) * var(--report-canvas-scale) * 1px);
      height: calc(var(--report-canvas-height) * var(--report-canvas-scale) * 1px);
      min-width: 100%;
    }

    ::slotted(.canvas-visual) {
      position: absolute;
      display: block;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      box-sizing: border-box;
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('ld-report-zoom-command', this.onZoomCommand as EventListener)
    this.resizeObserver = new ResizeObserver(() => this.updateScale())
    this.updateComplete.then(() => {
      this.resizeObserver?.observe(this)
      this.updateScale()
      this.positionVisuals()
      this.emitZoomState()
    })
  }

  disconnectedCallback(): void {
    document.removeEventListener('ld-report-zoom-command', this.onZoomCommand as EventListener)
    this.resizeObserver?.disconnect()
    super.disconnectedCallback()
  }

  updated(): void {
    this.updateScale()
    this.positionVisuals()
  }

  private updateScale(): void {
    const hostRect = this.getBoundingClientRect()
    const availableWidth = Math.max(0, hostRect.width)
    if (!availableWidth || !this.width) return
    const widthScale = availableWidth / this.width
    let nextScale = widthScale
    if (this.zoomMode === 'custom') {
      nextScale = this.customScale
    }
    nextScale = clampScale(nextScale)
    if (Math.abs(nextScale - this.scale) > 0.001) {
      this.scale = nextScale
      this.emitZoomState()
    }
  }

  private positionVisuals(): void {
    const slot = this.shadowRoot?.querySelector('slot:not([name])') as HTMLSlotElement | null
    const assigned = slot?.assignedElements({ flatten: true }) ?? []
    for (const element of assigned) {
      if (!(element instanceof HTMLElement)) continue
      this.positionVisual(element as VisualElement)
    }
  }

  private positionVisual(element: VisualElement): void {
    const x = parseCanvasNumber(element.dataset.x, 0)
    const y = parseCanvasNumber(element.dataset.y, 0)
    const width = parseCanvasNumber(element.dataset.w, 280)
    const height = parseCanvasNumber(element.dataset.h, 180)
    element.style.left = `${x}px`
    element.style.top = `${y}px`
    element.style.width = `${width}px`
    element.style.height = `${height}px`
  }

  private setZoomMode(mode: ZoomMode): void {
    this.zoomMode = mode
    try {
      localStorage.setItem(zoomStorageKey(), mode)
    } catch {
      // Ignore storage failures; the active component state still updates.
    }
    this.updateComplete.then(() => this.updateScale())
    this.updateComplete.then(() => this.emitZoomState())
  }

  private onZoomCommand = (event: CustomEvent<ZoomCommand>): void => {
    const detail = event.detail ?? {}
    if (detail.scale !== undefined) {
      this.customScale = clampScale(detail.scale)
      try {
        localStorage.setItem(zoomScaleStorageKey(), String(this.customScale))
      } catch {
        // Ignore storage failures; the active component state still updates.
      }
    }
    this.setZoomMode(detail.mode ?? (detail.scale !== undefined ? 'custom' : this.zoomMode))
  }

  private emitZoomState(): void {
    this.dispatchEvent(new CustomEvent('ld-report-zoom-state', {
      detail: { mode: this.zoomMode, scale: this.scale },
      bubbles: true,
      composed: true,
    }))
  }

  render() {
    const style = [
      `--report-canvas-width:${this.width}`,
      `--report-canvas-height:${this.height}`,
      `--report-canvas-scale:${this.scale}`,
    ].join(';')

    return html`
      <div class="surface" style=${style}>
        <div class="viewport">
          <div class="sizer">
            <div class="frame">
              <slot @slotchange=${this.positionVisuals}></slot>
            </div>
          </div>
        </div>
      </div>
    `
  }
}

class ReportZoom extends LitElement {
  @state() private mode: ZoomMode = storedZoomMode()
  @state() private scale = storedCustomScale()

  static styles = css`
    :host {
      display: inline-block;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    .zoom {
      display: inline-grid;
      grid-template-columns: auto auto minmax(132px, 190px) auto auto;
      align-items: center;
      overflow: hidden;
      border: 1px solid var(--borderColor-default);
      border-radius: 5px;
      background: var(--report-panel, var(--card-bgColor, var(--bgColor-default)));
    }

    button {
      display: grid;
      width: 30px;
      height: 30px;
      place-items: center;
      border: 0;
      border-left: 1px solid var(--borderColor-muted);
      background: transparent;
      color: var(--fgColor-muted);
      cursor: pointer;
      padding: 0;
      font: inherit;
    }

    button:first-child {
      border-left: 0;
    }

    button:hover,
    button:focus-visible {
      background: var(--bgColor-muted);
      color: var(--fgColor-default);
      outline: 0;
    }

    button[aria-pressed='true'] {
      background: var(--ld-accent);
      color: var(--ld-accent-fg);
    }

    svg {
      width: 15px;
      height: 15px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 2;
    }

    input {
      width: 100%;
      min-width: 0;
      accent-color: var(--ld-accent);
    }

    .slider {
      display: grid;
      min-width: 0;
      border-left: 1px solid var(--borderColor-muted);
      padding: 0 8px;
    }

    .percent {
      min-width: 45px;
      border-left: 1px solid var(--borderColor-muted);
      color: var(--fgColor-muted);
      text-align: center;
      font-size: 0.7rem;
      font-weight: 850;
      white-space: nowrap;
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('ld-report-zoom-state', this.onZoomState as EventListener)
  }

  disconnectedCallback(): void {
    document.removeEventListener('ld-report-zoom-state', this.onZoomState as EventListener)
    super.disconnectedCallback()
  }

  private onZoomState = (event: CustomEvent<{ mode: ZoomMode; scale: number }>): void => {
    this.mode = event.detail.mode
    this.scale = event.detail.scale
  }

  private command(detail: ZoomCommand): void {
    this.dispatchEvent(new CustomEvent('ld-report-zoom-command', {
      detail,
      bubbles: true,
      composed: true,
    }))
  }

  private nudge(delta: number): void {
    this.command({ mode: 'custom', scale: clampScale(this.scale + delta) })
  }

  private slide(event: Event): void {
    const input = event.currentTarget as HTMLInputElement
    this.command({ mode: 'custom', scale: clampScale(Number(input.value) / 100) })
  }

  render() {
    const percent = Math.round(this.scale * 100)
    return html`
      <div class="zoom" role="group" aria-label="Report zoom">
        <button type="button" title="Fit page" aria-label="Fit page" aria-pressed=${String(this.mode === 'fit-page')} @click=${() => this.command({ mode: 'fit-page' })}>
          ${zoomIcon('fit-page')}
        </button>
        <button type="button" title="Zoom out" aria-label="Zoom out" @click=${() => this.nudge(-0.1)}>
          ${zoomIcon('minus')}
        </button>
        <div class="slider">
          <input type="range" min="0" max="200" .value=${String(percent)} aria-label="Zoom percent" @input=${this.slide} />
        </div>
        <button type="button" title="Zoom in" aria-label="Zoom in" @click=${() => this.nudge(0.1)}>
          ${zoomIcon('plus')}
        </button>
        <span class="percent">${percent}%</span>
      </div>
    `
  }
}

function parseCanvasNumber(value: string | undefined, fallback: number): number {
  if (!value) return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

customElements.define('ld-report-canvas', ReportCanvas)
customElements.define('ld-report-zoom', ReportZoom)

function zoomStorageKey(): string {
  return `libredash-report-zoom:${location.pathname}`
}

function zoomScaleStorageKey(): string {
  return `libredash-report-zoom-scale:${location.pathname}`
}

function storedZoomMode(): ZoomMode {
  try {
    const value = localStorage.getItem(zoomStorageKey())
    if (value === 'custom') {
      return value
    }
  } catch {
    // Ignore storage failures.
  }
  return 'fit-page'
}

function storedCustomScale(): number {
  try {
    return clampScale(Number(localStorage.getItem(zoomScaleStorageKey()) || 0.6))
  } catch {
    return 0.6
  }
}

function clampScale(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.min(2, Math.max(0, value))
}

function zoomIcon(name: 'fit-page' | 'minus' | 'plus') {
  switch (name) {
    case 'fit-page':
      return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7V5a2 2 0 0 1 2-2h2"></path><path d="M17 3h2a2 2 0 0 1 2 2v2"></path><path d="M21 17v2a2 2 0 0 1-2 2h-2"></path><path d="M7 21H5a2 2 0 0 1-2-2v-2"></path></svg>`
    case 'minus':
      return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"></path></svg>`
    case 'plus':
      return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14"></path><path d="M5 12h14"></path></svg>`
  }
}
