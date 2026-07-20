import { LitElement, css, html } from 'lit'
import { ChevronRight, LayoutDashboard } from 'lucide'
import type { CatalogPageSignal } from '../../generated/signals'
import { catalogListStyles } from '../shared/catalog-list-styles'
import { DatastarLit } from '../shared/datastar-lit'
import { checkSignalContract } from '../shared/signal-contract'
import { lucideIcon } from '../shared/lucide-icons'

class LibreDashCatalogPage extends DatastarLit(LitElement) {
  static styles = [catalogListStyles, css`
    :host {
      display: block;
      min-width: 0;
      min-height: 100svh;
      background: var(--ld-bg-app);
      color: var(--ld-fg-default);
      font-family: var(--ld-font-family-ui, var(--fontStack-system));
    }

    section {
      display: grid;
      width: min(100%, var(--ld-page-content-max-width));
      min-width: 0;
      min-height: 100svh;
      align-content: start;
      gap: var(--base-size-16);
      box-sizing: border-box;
      margin-inline: auto;
      padding: var(--base-size-16);
    }

    header {
      min-width: 0;
    }

    h1,
    p {
      margin: 0;
    }

    h1 {
      overflow: hidden;
      color: var(--ld-fg-default);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--ld-font-size-title-sm);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-compact);
    }

    .detail {
      margin-top: var(--base-size-4);
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-body-sm);
      line-height: var(--ld-line-height-snug);
    }

    .dashboard-icon {
      border-color: var(--ld-asset-dashboard-border);
      background: var(--ld-asset-dashboard-bg);
      color: var(--ld-asset-dashboard-accent);
    }
  `]

  updated(): void {
    const page = this.page
    if (!page) return
    checkSignalContract('catalog page', page, { kind: 'required', dashboards: 'required' })
  }

  get page(): CatalogPageSignal | null {
    return this.signal<CatalogPageSignal | null>('page', null)
  }

  render() {
    const page = this.page
    if (!page) return html`<slot></slot>`
    return html`
      <section aria-label="LibreDash dashboard catalog">
        <header>
          <h1>${page.title}</h1>
          <p class="detail">${page.description}</p>
        </header>
        <ul class="catalog-list dashboard-list" aria-label="Published dashboards">
          ${page.dashboards.map((dashboard) => html`
            <li>
              <a class="catalog-row dashboard-row" href=${dashboard.href}>
                <span class="catalog-icon dashboard-icon">${lucideIcon(LayoutDashboard)}</span>
                <span class="catalog-copy dashboard-copy">
                  <span class="catalog-title dashboard-title">${dashboard.title}</span>
                  <span class="catalog-description dashboard-description">${dashboard.description || dashboard.semanticModel || 'Dashboard'}</span>
                </span>
                <span class="catalog-trailing">
                  <span class="catalog-meta dashboard-pages">${dashboard.pageCount} ${dashboard.pageCount === 1 ? 'page' : 'pages'}</span>
                  <span class="catalog-chevron dashboard-chevron">${lucideIcon(ChevronRight)}</span>
                </span>
              </a>
            </li>
          `)}
        </ul>
      </section>
    `
  }
}

customElements.define('ld-catalog-page', LibreDashCatalogPage)
