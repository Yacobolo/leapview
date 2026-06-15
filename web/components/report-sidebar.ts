import { LitElement, css, html, nothing, svg as svgTemplate } from 'lit'
import { property, state } from 'lit/decorators.js'

type ReportPage = {
  id: string
  title: string
  href: string
  active?: boolean
}

type ReportSidebarConfig = {
  dashboardId?: string
  dashboardTitle?: string
  pageId?: string
  pageTitle?: string
  modelId?: string
  modelTitle?: string
  modelHref?: string
  pages?: ReportPage[]
}

const defaultConfig: ReportSidebarConfig = {
  dashboardTitle: 'Dashboard',
  pageTitle: 'Page',
  pages: [],
}

const configConverter = {
  fromAttribute(value: string | null): ReportSidebarConfig {
    if (!value) return defaultConfig
    try {
      return { ...defaultConfig, ...JSON.parse(value) } as ReportSidebarConfig
    } catch {
      return defaultConfig
    }
  },
  toAttribute(value: ReportSidebarConfig): string {
    return JSON.stringify(value ?? defaultConfig)
  },
}

class ReportSidebar extends LitElement {
  @property({ attribute: 'config', converter: configConverter }) config: ReportSidebarConfig = defaultConfig
  @state() private collapsed = storedCollapsed()

  static styles = css`
    :host {
      --ld-report-sidebar-width: 176px;
      display: block;
      width: var(--ld-report-sidebar-width);
      min-height: 100svh;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      transition: width 180ms var(--ld-ease-out);
    }

    :host([data-collapsed]) {
      --ld-report-sidebar-width: 44px;
    }

    aside {
      position: sticky;
      top: 0;
      display: grid;
      width: var(--ld-report-sidebar-width);
      min-height: 100svh;
      grid-template-rows: auto minmax(0, 1fr) auto;
      border-right: 1px solid var(--borderColor-default);
      background: color-mix(in srgb, var(--bgColor-muted), var(--bgColor-default) 42%);
      transition: width 180ms var(--ld-ease-out);
    }

    header {
      display: grid;
      gap: 6px;
      min-width: 0;
      border-bottom: 1px solid var(--borderColor-muted);
      padding: 11px 8px 10px;
    }

    .top-row {
      display: flex;
      min-width: 0;
      align-items: center;
      gap: 8px;
    }

    .glyph,
    .page-initial {
      display: grid;
      width: 26px;
      height: 26px;
      flex: 0 0 auto;
      place-items: center;
      border-radius: 6px;
      background: transparent;
      color: var(--fgColor-muted);
      font-size: 0.65rem;
      font-weight: 900;
    }

    .glyph svg,
    .model-link svg,
    .collapse svg {
      width: 15px;
      height: 15px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 2;
    }

    .titles {
      display: grid;
      gap: 2px;
      min-width: 0;
    }

    .eyebrow,
    .model-label {
      overflow: hidden;
      color: var(--fgColor-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.58rem;
      font-weight: 950;
      letter-spacing: 0;
      text-transform: uppercase;
    }

    .dashboard-title,
    .page-title {
      overflow: hidden;
      color: var(--fgColor-default);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 850;
      letter-spacing: 0;
    }

    .dashboard-title {
      font-size: 0.75rem;
    }

    .page-title {
      font-size: 0.68rem;
      color: var(--fgColor-muted);
    }

    .collapse {
      display: grid;
      width: 26px;
      height: 26px;
      flex: 0 0 auto;
      place-items: center;
      margin-left: auto;
      border: 1px solid transparent;
      border-radius: 6px;
      background: transparent;
      color: var(--fgColor-muted);
      cursor: pointer;
      padding: 0;
    }

    .collapse:hover,
    .collapse:focus-visible {
      border-color: var(--borderColor-muted);
      background: var(--bgColor-muted);
      color: var(--fgColor-default);
      outline: 0;
    }

    nav {
      display: grid;
      align-content: start;
      gap: 5px;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      padding: 9px 5px;
    }

    a {
      text-decoration: none;
    }

    .page-link,
    .model-link {
      position: relative;
      display: grid;
      grid-template-columns: 26px minmax(0, 1fr);
      min-height: 32px;
      align-items: center;
      gap: 8px;
      border: 1px solid transparent;
      border-radius: 7px;
      color: var(--fgColor-muted);
      padding: 0 8px;
      font-size: 0.72rem;
      font-weight: 800;
    }

    .page-link:hover,
    .page-link:focus-visible,
    .model-link:hover,
    .model-link:focus-visible {
      background: var(--bgColor-muted);
      color: var(--fgColor-default);
      outline: 0;
    }

    .page-link[aria-current='page'] {
      border-color: transparent;
      background: color-mix(in srgb, var(--bgColor-muted), var(--bgColor-default) 34%);
      color: var(--fgColor-default);
    }

    .page-link[aria-current='page']::before {
      content: '';
      position: absolute;
      inset-block: 7px;
      left: 0;
      width: 2px;
      border-radius: 999px;
      background: var(--ld-accent);
    }

    .link-text {
      overflow: hidden;
      min-width: 0;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    footer {
      display: grid;
      gap: 6px;
      border-top: 1px solid var(--borderColor-muted);
      padding: 7px 5px 8px;
    }

    :host([data-collapsed]) header {
      padding-inline: 6px;
    }

    :host([data-collapsed]) .titles,
    :host([data-collapsed]) .link-text,
    :host([data-collapsed]) .model-label {
      display: none;
    }

    :host([data-collapsed]) .top-row {
      display: grid;
      justify-items: center;
    }

    :host([data-collapsed]) .collapse {
      margin-left: 0;
    }

    :host([data-collapsed]) .page-link,
    :host([data-collapsed]) .model-link {
      grid-template-columns: 26px;
      justify-content: center;
      padding-inline: 0;
    }

    :host([data-collapsed]) .page-link[aria-current='page']::before {
      content: none;
    }
  `

  updated(): void {
    this.toggleAttribute('data-collapsed', this.collapsed)
  }

  render() {
    const pages = this.config.pages ?? []
    return html`
      <aside aria-label="Report pages">
        <header>
          <div class="top-row">
            <span class="glyph">${icon('report')}</span>
            <div class="titles">
              <span class="eyebrow">Report</span>
              <strong class="dashboard-title" title=${this.config.dashboardTitle || ''}>${this.config.dashboardTitle || 'Dashboard'}</strong>
              <span class="page-title" title=${this.config.pageTitle || ''}>${this.config.pageTitle || ''}</span>
            </div>
            <button
              class="collapse"
              type="button"
              aria-label=${this.collapsed ? 'Expand report pages' : 'Collapse report pages'}
              aria-pressed=${String(this.collapsed)}
              title=${this.collapsed ? 'Expand report pages' : 'Collapse report pages'}
              @click=${this.toggleCollapsed}
            >
              ${icon(this.collapsed ? 'expand' : 'collapse')}
            </button>
          </div>
        </header>

        <nav aria-label="Report pages">
          ${pages.map((page) => this.renderPageLink(page))}
        </nav>

        <footer>
          <span class="model-label">Semantic model</span>
          ${this.config.modelHref ? html`
            <a class="model-link" href=${this.config.modelHref} title=${this.config.modelTitle || 'Semantic model'}>
              <span class="glyph">${icon('model')}</span>
              <span class="link-text">${this.config.modelTitle || this.config.modelId || 'Model'}</span>
            </a>
          ` : nothing}
        </footer>
      </aside>
    `
  }

  private renderPageLink(page: ReportPage) {
    const active = Boolean(page.active || page.id === this.config.pageId)
    const title = page.title || page.id
    return html`
      <a class="page-link" href=${page.href} aria-current=${active ? 'page' : 'false'} title=${title}>
        <span class="page-initial" aria-hidden="true">${initials(title)}</span>
        <span class="link-text">${title}</span>
      </a>
    `
  }

  private toggleCollapsed = () => {
    this.collapsed = !this.collapsed
    try {
      localStorage.setItem('libredash-report-sidebar-collapsed', String(this.collapsed))
    } catch {
      // Session state still updates when storage is unavailable.
    }
  }
}

function initials(value: string): string {
  const words = value.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return 'P'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return words.slice(0, 2).map((word) => word[0]).join('').toUpperCase()
}

function storedCollapsed(): boolean {
  try {
    return localStorage.getItem('libredash-report-sidebar-collapsed') === 'true'
  } catch {
    return false
  }
}

function icon(name: 'report' | 'model' | 'collapse' | 'expand') {
  switch (name) {
    case 'report':
      return iconSvg(svgTemplate`<path d="M3 3v18h18"></path><path d="M8 17V9"></path><path d="M13 17V5"></path><path d="M18 17v-6"></path>`)
    case 'model':
      return iconSvg(svgTemplate`<ellipse cx="12" cy="5" rx="8" ry="3"></ellipse><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"></path><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"></path>`)
    case 'collapse':
      return iconSvg(svgTemplate`<rect x="3" y="4" width="18" height="16" rx="2"></rect><path d="M9 4v16"></path><path d="m16 10-2 2 2 2"></path>`)
    case 'expand':
      return iconSvg(svgTemplate`<rect x="3" y="4" width="18" height="16" rx="2"></rect><path d="M9 4v16"></path><path d="m14 10 2 2-2 2"></path>`)
  }
}

function iconSvg(content: unknown) {
  return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`
}

customElements.define('ld-report-sidebar', ReportSidebar)
