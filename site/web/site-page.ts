import { LitElement, css, html } from 'lit'
import { Blocks, Boxes, ChartNoAxesCombined, Check, Copy, Database, GitBranch, Menu, Monitor, Moon, PanelLeftClose, PanelLeftOpen, Radio, Server, Sun, X, type IconNode } from 'lucide'
import { DatastarLit } from '../../web/components/shared/datastar-lit'
import { lucideIcon } from '../../web/components/shared/lucide-icons'
import type { ChartPayload } from '../../web/components/dashboard/charts/types'
import type { TableSignal } from '../../web/components/dashboard/table/types'

type DemoSignal = {
  chart?: ChartPayload
}

type ThemeMode = 'system' | 'light' | 'dark'

const nextThemeMode: Record<ThemeMode, ThemeMode> = {
  system: 'light',
  light: 'dark',
  dark: 'system',
}

const themeLabels: Record<ThemeMode, string> = {
  system: 'System theme',
  light: 'Light theme',
  dark: 'Dark theme',
}

class SiteThemeToggle extends LitElement {
  private themeMode: ThemeMode = currentThemeMode()
  private readonly handleThemeApplied = (event: Event) => {
    this.themeMode = normalizeThemeMode((event as CustomEvent<{ mode?: string }>).detail?.mode)
    this.requestUpdate()
  }

  static styles = css`
    :host {
      display: block;
    }

    button {
      display: inline-grid;
      width: var(--site-header-control-size);
      height: var(--site-header-control-size);
      place-items: center;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      color: var(--ld-fg-muted);
      cursor: pointer;
      font: inherit;
    }

    button:hover,
    button:focus-visible {
      border-color: var(--ld-button-border-hover);
      background: var(--ld-button-bg-hover);
      color: var(--ld-fg-default);
    }

    button:focus-visible {
      outline: var(--focus-outline);
      outline-offset: var(--focus-outline-offset);
    }

    [hidden] {
      display: none;
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('libredash-theme-applied', this.handleThemeApplied)
  }

  disconnectedCallback(): void {
    document.removeEventListener('libredash-theme-applied', this.handleThemeApplied)
    super.disconnectedCallback()
  }

  render() {
    const nextMode = nextThemeMode[this.themeMode]
    const label = `${themeLabels[this.themeMode]}. Switch to ${themeLabels[nextMode]}.`
    return html`<button
      type="button"
      data-theme-toggle
      data-theme-mode=${this.themeMode}
      aria-label=${label}
      title=${label}
      @click=${this.toggleTheme}
    >
      <span data-theme-icon="system" ?hidden=${this.themeMode !== 'system'}>${lucideIcon(Monitor)}</span>
      <span data-theme-icon="light" ?hidden=${this.themeMode !== 'light'}>${lucideIcon(Sun)}</span>
      <span data-theme-icon="dark" ?hidden=${this.themeMode !== 'dark'}>${lucideIcon(Moon)}</span>
    </button>`
  }

  private toggleTheme(): void {
    const nextMode = nextThemeMode[this.themeMode]
    this.themeMode = nextMode
    this.requestUpdate()
    document.dispatchEvent(new CustomEvent('libredash-theme-change', { detail: { mode: nextMode } }))
  }
}

if (!customElements.get('ld-site-theme-toggle')) {
  customElements.define('ld-site-theme-toggle', SiteThemeToggle)
}

class SiteMobileMenu extends LitElement {
  private open = false

  static styles = css`
    :host {
      display: none;
    }

    @media (width < 48rem) {
      :host {
        display: block;
      }
    }

    button {
      display: inline-grid;
      width: var(--site-header-control-size);
      height: var(--site-header-control-size);
      place-items: center;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      color: var(--ld-fg-muted);
      cursor: pointer;
      font: inherit;
    }

    button:hover,
    button:focus-visible {
      border-color: var(--ld-button-border-hover);
      background: var(--ld-button-bg-hover);
      color: var(--ld-fg-default);
    }

    button:focus-visible {
      outline: var(--focus-outline);
      outline-offset: var(--focus-outline-offset);
    }

    nav {
      position: fixed;
      z-index: var(--zIndex-overlay);
      top: calc(var(--site-header-height) + var(--base-size-8));
      right: var(--base-size-16);
      display: grid;
      min-width: calc(var(--base-size-128) + var(--base-size-64));
      overflow: hidden;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-large);
      background: var(--ld-bg-panel);
      box-shadow: var(--shadow-floating-medium);
    }

    a {
      padding: var(--base-size-12) var(--base-size-16);
      color: var(--ld-fg-default);
      font-size: var(--ld-text-body-md-size);
      font-weight: var(--ld-font-weight-medium);
      text-decoration: none;
    }

    a:hover,
    a:focus-visible {
      background: var(--ld-bg-control);
      color: var(--ld-fg-accent);
    }

    nav[hidden] {
      display: none;
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('keydown', this.handleKeydown)
  }

  disconnectedCallback(): void {
    document.removeEventListener('keydown', this.handleKeydown)
    super.disconnectedCallback()
  }

  render() {
    const label = this.open ? 'Close site navigation' : 'Open site navigation'
    return html`<button type="button" aria-label=${label} aria-controls="site-mobile-navigation" aria-expanded=${String(this.open)} @click=${this.toggle}>
      ${lucideIcon(this.open ? X : Menu, { size: 20, strokeWidth: 2 })}
    </button>
    <nav id="site-mobile-navigation" aria-label="Site navigation" ?hidden=${!this.open}>
      <a href="/docs" @click=${this.close}>Docs</a>
      <a href="/#demo" @click=${this.close}>Demo</a>
      <a href="/charts" @click=${this.close}>Charts</a>
    </nav>`
  }

  private toggle = (): void => {
    this.open = !this.open
    this.requestUpdate()
  }

  private close = (): void => {
    this.open = false
    this.requestUpdate()
  }

  private readonly handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) this.close()
  }
}

if (!customElements.get('ld-site-mobile-menu')) {
  customElements.define('ld-site-mobile-menu', SiteMobileMenu)
}

class SiteDocsDrawerToggle extends LitElement {
  static properties = {
    placement: { type: String },
  }

  declare placement: string

  private open = false
  private readonly handleDrawerState = (event: Event) => {
    this.open = Boolean((event as CustomEvent<{ open?: boolean }>).detail?.open)
    this.requestUpdate()
  }

  static styles = css`
    :host {
      display: none;
    }

    @media (width < 48rem) {
      :host {
        display: block;
      }
    }

    button {
      display: inline-grid;
      width: var(--site-header-control-size);
      height: var(--site-header-control-size);
      place-items: center;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      color: var(--ld-fg-muted);
      cursor: pointer;
      font: inherit;
    }

    button:hover,
    button:focus-visible {
      border-color: var(--ld-button-border-hover);
      background: var(--ld-button-bg-hover);
      color: var(--ld-fg-default);
    }

    button:focus-visible {
      outline: var(--focus-outline);
      outline-offset: var(--focus-outline-offset);
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('libredash-docs-drawer-state', this.handleDrawerState)
  }

  disconnectedCallback(): void {
    document.removeEventListener('libredash-docs-drawer-state', this.handleDrawerState)
    super.disconnectedCallback()
  }

  render() {
    const closeControl = this.placement === 'drawer'
    const label = closeControl || this.open ? 'Close documentation menu' : 'Open documentation menu'
    const icon = closeControl || this.open ? PanelLeftClose : PanelLeftOpen
    return html`<button
      type="button"
      aria-label=${label}
      aria-controls="site-docs-sidebar"
      aria-expanded=${String(this.open)}
      @click=${this.toggleDrawer}
    >${lucideIcon(closeControl ? X : icon, { size: 18, strokeWidth: 2 })}</button>`
  }

  private toggleDrawer = (): void => {
    document.dispatchEvent(new CustomEvent('libredash-docs-drawer-request', {
      detail: { open: this.placement === 'drawer' ? false : !this.open },
    }))
  }
}

if (!customElements.get('ld-site-docs-drawer-toggle')) {
  customElements.define('ld-site-docs-drawer-toggle', SiteDocsDrawerToggle)
}

function syncDocsDrawer(open = false): void {
  const layout = document.querySelector<HTMLElement>('.site-docs-layout')
  const sidebar = document.querySelector<HTMLElement>('.site-docs-sidebar')
  if (!layout || !sidebar) return

  const compact = window.matchMedia('(width < 48rem)').matches
  const nextOpen = compact && open
  const wasOpen = layout.classList.contains('site-docs-drawer-open')
  layout.classList.toggle('site-docs-drawer-open', nextOpen)
  sidebar.inert = compact && !nextOpen
  sidebar.setAttribute('aria-hidden', String(compact && !nextOpen))
  document.body.classList.toggle('site-docs-drawer-open', nextOpen)
  document.dispatchEvent(new CustomEvent('libredash-docs-drawer-state', { detail: { open: nextOpen } }))
  if (compact && wasOpen && !nextOpen) {
    document.querySelector<HTMLElement>('ld-site-docs-drawer-toggle:not([placement])')?.shadowRoot?.querySelector<HTMLButtonElement>('button')?.focus()
  }
}

document.addEventListener('libredash-docs-drawer-request', (event) => {
  const requested = (event as CustomEvent<{ open?: boolean }>).detail?.open
  const currentlyOpen = document.querySelector('.site-docs-layout')?.classList.contains('site-docs-drawer-open') ?? false
  syncDocsDrawer(typeof requested === 'boolean' ? requested : !currentlyOpen)
})

document.addEventListener('click', (event) => {
  if ((event.target as Element).closest('[data-site-docs-drawer-close]')) syncDocsDrawer(false)
})

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') syncDocsDrawer(false)
})

window.addEventListener('resize', () => syncDocsDrawer(document.querySelector('.site-docs-layout')?.classList.contains('site-docs-drawer-open')))
syncDocsDrawer()

class SiteMarkdownCopy extends LitElement {
  static properties = {
    markdown: { type: String },
  }

  declare markdown: string

  private copied = false
  private resetTimer?: number

  static styles = css`
    :host {
      display: inline-block;
    }

    button {
      display: inline-flex;
      min-height: var(--control-medium-size);
      align-items: center;
      gap: var(--base-size-8);
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      color: var(--ld-fg-default);
      cursor: pointer;
      font: inherit;
      font-size: var(--ld-text-body-sm-size);
      font-weight: var(--ld-font-weight-medium);
      padding: 0 var(--base-size-12);
    }

    button:hover,
    button:focus-visible {
      border-color: var(--ld-button-border-hover);
      background: var(--ld-button-bg-hover);
    }

    button:focus-visible {
      outline: var(--focus-outline);
      outline-offset: var(--focus-outline-offset);
    }
  `

  disconnectedCallback(): void {
    window.clearTimeout(this.resetTimer)
    super.disconnectedCallback()
  }

  render() {
    const label = this.copied ? 'Markdown copied' : 'Copy Markdown'
    return html`<button type="button" aria-label=${label} @click=${this.copyMarkdown}>
      ${lucideIcon(this.copied ? Check : Copy, { size: 16, strokeWidth: 2 })}
      <span>${this.copied ? 'Copied' : 'Copy Markdown'}</span>
    </button>`
  }

  private copyMarkdown = async (): Promise<void> => {
    if (!this.markdown) return

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(this.markdown)
      } else {
        this.copyWithSelection()
      }
    } catch {
      return
    }

    this.copied = true
    this.requestUpdate()
    window.clearTimeout(this.resetTimer)
    this.resetTimer = window.setTimeout(() => {
      this.copied = false
      this.requestUpdate()
    }, 2_000)
  }

  private copyWithSelection(): void {
    const textarea = document.createElement('textarea')
    textarea.value = this.markdown
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.append(textarea)
    textarea.select()

    const copied = document.execCommand('copy')
    textarea.remove()
    if (!copied) throw new Error('clipboard write failed')
  }
}

if (!customElements.get('ld-site-markdown-copy')) {
  customElements.define('ld-site-markdown-copy', SiteMarkdownCopy)
}

const featureIcons: Record<string, IconNode> = {
  blocks: Blocks,
	boxes: Boxes,
  chart: ChartNoAxesCombined,
  database: Database,
  'git-branch': GitBranch,
  radio: Radio,
	server: Server,
}

class SiteFeatureIcon extends LitElement {
  static properties = {
    name: { type: String },
  }

  declare name: string

  static styles = css`
    :host {
      display: grid;
      width: var(--control-large-size);
      height: var(--control-large-size);
      place-items: center;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-large);
      background: var(--ld-bg-control);
      color: var(--ld-fg-accent);
    }
  `

  render() {
    return lucideIcon(featureIcons[this.name] ?? Blocks, { size: 22, strokeWidth: 1.8 })
  }
}

if (!customElements.get('ld-site-feature-icon')) {
  customElements.define('ld-site-feature-icon', SiteFeatureIcon)
}

function currentThemeMode(): ThemeMode {
  try {
    return normalizeThemeMode(localStorage.getItem('libredash-color-mode'))
  } catch {
    return normalizeThemeMode(document.documentElement.dataset.colorMode)
  }
}

function normalizeThemeMode(mode: string | null | undefined): ThemeMode {
	return mode === 'light' || mode === 'dark' || mode === 'system' ? mode : 'system'
}

class SiteChartDemo extends DatastarLit(LitElement) {
  static styles = css`
    :host {
      display: block;
      min-height: 28rem;
    }

    ld-echart {
      height: 28rem;
    }
  `

  render() {
    const demo = this.signal<DemoSignal>('demo', {})
    return html`<ld-echart .chart=${demo.chart ?? null}></ld-echart>`
  }
}

if (!customElements.get('ld-site-chart-demo')) {
  customElements.define('ld-site-chart-demo', SiteChartDemo)
}

type ArticleSection = { id: string; label: string; level: number }

class SiteArticleToc extends LitElement {
  private sections: ArticleSection[] = []
  private activeId = ''
  private observer?: IntersectionObserver

  static styles = css`
    :host { display: block; position: sticky; top: calc(var(--control-xlarge-size) + var(--base-size-32)); align-self: start; max-height: calc(100svh - var(--control-xlarge-size) - var(--base-size-64)); overflow: auto; }
    nav { display: grid; gap: var(--space-xs); border-left: var(--ld-border-muted); padding-left: var(--base-size-16); }
    h2 { margin: 0 0 var(--base-size-4); color: var(--ld-fg-muted); font-size: var(--ld-text-caption-size); font-weight: var(--ld-font-weight-strong); letter-spacing: var(--base-size-2); text-transform: uppercase; }
    a { color: var(--ld-fg-muted); font-size: var(--ld-text-body-sm-size); line-height: var(--ld-line-height-default); text-decoration: none; }
    a[data-level="3"] { padding-left: var(--base-size-8); }
    a:hover, a:focus-visible, a.active { color: var(--ld-fg-default); font-weight: var(--ld-font-weight-strong); }
    @media (width < 80rem) { :host { display: none; } }
  `

  connectedCallback() {
    super.connectedCallback()
    requestAnimationFrame(() => this.collectSections())
  }

  disconnectedCallback() { this.observer?.disconnect(); super.disconnectedCallback() }

  private collectSections() {
    const headings = Array.from(document.querySelectorAll<HTMLElement>('.site-docs-article h2, .site-docs-article h3'))
    const used = new Set<string>()
    this.sections = headings.map((heading) => {
      let id = heading.id || heading.textContent?.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'section'
      const base = id; let suffix = 2
      while (used.has(id)) id = `${base}-${suffix++}`
      used.add(id); heading.id = id
      return { id, label: heading.textContent?.trim() ?? '', level: Number(heading.tagName.slice(1)) }
    })
    this.activeId = this.sections[0]?.id ?? ''
    this.observer = new IntersectionObserver((entries) => {
      const visible = entries.filter((entry) => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0]
      if (visible?.target.id && this.activeId !== visible.target.id) { this.activeId = visible.target.id; this.requestUpdate() }
    }, { rootMargin: '-18% 0px -70% 0px', threshold: 0 })
    headings.forEach((heading) => this.observer?.observe(heading))
    this.requestUpdate()
  }

  render() { return this.sections.length ? html`<nav aria-label="In this article"><h2>In this article</h2>${this.sections.map((section) => html`<a class=${section.id === this.activeId ? 'active' : ''} data-level=${section.level} href=${`#${section.id}`}>${section.label}</a>`)}</nav>` : null }
}

if (!customElements.get('ld-site-article-toc')) customElements.define('ld-site-article-toc', SiteArticleToc)

class SiteDocsChart extends DatastarLit(LitElement) {
  static properties = {
    chartId: { type: String, attribute: 'chart-id' },
  }

  declare chartId: string

  static styles = css`
    :host {
      display: block;
      min-height: 28rem;
      margin-block: var(--base-size-24);
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-chart-surface);
      box-shadow: var(--shadow-resting-small);
      overflow: hidden;
    }

    ld-echart,
    ld-kpi-card {
      display: block;
      height: 28rem;
    }
  `

  render() {
    const charts = this.signal<ChartPayload[]>('charts', [])
    const chart = charts.find((candidate) => candidate.id === this.chartId) ?? null
    if (chart?.type === 'kpi') {
      return html`<ld-kpi-card .visual=${chart}></ld-kpi-card>`
    }
    return html`<ld-echart .chart=${chart}></ld-echart>`
  }
}

if (!customElements.get('ld-site-doc-chart')) {
  customElements.define('ld-site-doc-chart', SiteDocsChart)
}

class SiteChartShowcase extends DatastarLit(LitElement) {
  static styles = css`
    :host {
      display: block;
    }

    .showcase-section {
      display: grid;
      gap: var(--base-size-16);
    }

    .section-heading {
      display: grid;
      gap: var(--base-size-4);
    }

    h2,
    p {
      margin: 0;
    }

    h2 {
      color: var(--ld-fg-default);
      font-size: var(--ld-text-title-lg-size);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-tight);
    }

    p {
      color: var(--ld-fg-muted);
      font-size: var(--ld-text-body-md-size);
      line-height: var(--ld-line-height-relaxed);
    }

    .chart-grid,
    .table-grid {
      display: grid;
      gap: var(--base-size-16);
    }

    .chart-grid {
      grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
    }

    .table-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .chart {
      min-height: 20rem;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-chart-surface);
      box-shadow: var(--shadow-resting-small);
      overflow: hidden;
    }

    ld-echart,
    ld-kpi-card {
      display: block;
      height: 20rem;
    }

    .table-card {
      min-width: 0;
      height: 26rem;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-chart-surface);
      box-shadow: var(--shadow-resting-small);
      overflow: hidden;
    }

    .table-card.featured {
      grid-column: 1 / -1;
      height: 30rem;
    }

    ld-report-table {
      display: block;
      height: 100%;
    }

    @media (width < 48rem) {
      .table-grid {
        grid-template-columns: minmax(0, 1fr);
      }

      .table-card.featured {
        grid-column: auto;
      }
    }
  `

  render() {
    const charts = this.signal<ChartPayload[]>('charts', [])
    const tables = this.signal<TableSignal[]>('tables', [])
    return html`
      <section class="showcase-section" aria-labelledby="chart-showcase-heading">
        <div class="section-heading">
          <h2 id="chart-showcase-heading">Charts</h2>
          <p>Renderer-neutral chart payloads, adapted by the product ECharts plugin.</p>
        </div>
        <div class="chart-grid">
          ${charts.map((chart) => html`<article class="chart">
            ${chart.type === 'kpi'
              ? html`<ld-kpi-card .visual=${chart}></ld-kpi-card>`
              : html`<ld-echart .chart=${chart}></ld-echart>`}
          </article>`)}
        </div>
      </section>
      <section class="showcase-section" aria-labelledby="table-showcase-heading">
        <div class="section-heading">
          <h2 id="table-showcase-heading">Tables, matrices, and pivots</h2>
          <p>Table variants from the Visual Showcase dashboard, including density, grid, and conditional-formatting treatments.</p>
        </div>
        <div class="table-grid">
          ${tables.map((table, index) => html`<article class="table-card ${index === 0 ? 'featured' : ''}">
            <ld-report-table table-id=${table.title} .table=${table}></ld-report-table>
          </article>`)}
        </div>
      </section>
    `
  }
}

if (!customElements.get('ld-site-chart-showcase')) {
  customElements.define('ld-site-chart-showcase', SiteChartShowcase)
}

async function loadRouteComponents(): Promise<void> {
  const imports: Promise<unknown>[] = []
  if (document.querySelector('ld-site-chart-demo, ld-site-chart-showcase, ld-site-doc-chart')) {
    imports.push(import('../../web/components/dashboard/charts/echart'))
  }
  if (document.querySelector('ld-site-chart-showcase')) {
    imports.push(import('../../web/components/dashboard/table/report-table'))
  }
  if (document.querySelector('ld-topology-background')) {
    imports.push(import('../../web/components/login/topology-background'))
  }
  await Promise.all(imports)
}

void loadRouteComponents()
