import { LitElement, css, html } from 'lit'
import { property, state } from 'lit/decorators.js'

type VisualElement = HTMLElement & {
  dataset: DOMStringMap
}

class ReportCanvas extends LitElement {
  @property({ type: Number }) width = 1366
  @property({ type: Number }) height = 768
  @state() private scale = 1

  private resizeObserver?: ResizeObserver

  static styles = css`
    :host {
      display: block;
      width: 100%;
      max-width: 100%;
      min-width: 0;
      box-sizing: border-box;
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
      width: calc(var(--report-canvas-width) * 1px);
      height: calc(var(--report-canvas-height) * 1px);
      transform: scale(var(--report-canvas-scale));
      transform-origin: top left;
      background:
        linear-gradient(var(--report-page-bg), var(--report-page-bg)),
        radial-gradient(circle at 1px 1px, var(--report-grid-dot) 1px, transparent 0);
      background-size: auto, 16px 16px;
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
    this.resizeObserver = new ResizeObserver(() => this.updateScale())
    this.updateComplete.then(() => {
      this.resizeObserver?.observe(this)
      this.updateScale()
      this.positionVisuals()
    })
  }

  disconnectedCallback(): void {
    this.resizeObserver?.disconnect()
    super.disconnectedCallback()
  }

  updated(): void {
    this.updateScale()
    this.positionVisuals()
  }

  private updateScale(): void {
    const availableWidth = this.getBoundingClientRect().width
    if (!availableWidth || !this.width) return
    const nextScale = Math.min(1, Math.max(0.42, availableWidth / this.width))
    if (Math.abs(nextScale - this.scale) > 0.001) {
      this.scale = nextScale
    }
  }

  private positionVisuals(): void {
    const slot = this.shadowRoot?.querySelector('slot') as HTMLSlotElement | null
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

  render() {
    const style = [
      `--report-canvas-width:${this.width}`,
      `--report-canvas-height:${this.height}`,
      `--report-canvas-scale:${this.scale}`,
    ].join(';')

    return html`
      <div class="viewport" style=${style}>
        <div class="sizer">
          <div class="frame">
            <slot @slotchange=${this.positionVisuals}></slot>
          </div>
        </div>
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
