import { LitElement, css, html, nothing } from 'lit'
import { property } from 'lit/decorators.js'
import { ArrowLeft, ExternalLink, FileText, Search } from 'lucide'
import type {
  ConnectionsPageSignal,
  DefinitionFactSignal,
  MetricGridSignal,
  WorkspaceAccessSignal,
  WorkspaceAssetPageSignal,
  WorkspaceAssetSummarySignal,
  WorkspaceDetailSectionSignal,
  WorkspacePageSignal,
  WorkspaceTabSignal,
} from '../../generated/signals'
import { jsonAttribute } from '../shared/json-attribute'
import { checkSignalContract } from '../shared/signal-contract'
import { lucideIcon } from '../shared/lucide-icons'
import '../shared/data-grid'
import '../shared/code-block'
import '../shared/workspace-access-control'

const emptyWorkspaceAccess: WorkspaceAccessSignal = {
  workspace: {},
  roles: [],
  bindings: [],
  canManage: false,
  status: { loading: false, error: '', message: '' },
  csrfToken: '',
  command: { email: '', role: '', principalId: '' },
  search: '',
}

class LibreDashWorkspacePage extends LitElement {
  @property({ converter: jsonAttribute<WorkspacePageSignal | null>(null) }) page: WorkspacePageSignal | null = null
  @property({ attribute: 'workspaceaccess', converter: jsonAttribute<WorkspaceAccessSignal>(emptyWorkspaceAccess) }) workspaceAccess: WorkspaceAccessSignal = emptyWorkspaceAccess

  static styles = workspaceStyles

  updated(): void {
    checkSignalContract('workspace page', this.page, { kind: 'required', title: 'required' })
  }

  render() {
    const page = this.page
    if (!page) return html`<slot></slot>`
    if (page.cards?.length) return this.renderCatalog(page)
    if (!page.assetList?.searchHref && this.workspaceAccess?.canManage) return this.renderAccessPage(page)
    return this.renderAssetList(page, 'Workspace', 'Workspace assets')
  }

  private renderCatalog(page: WorkspacePageSignal) {
    return html`
      <section class="page catalog" aria-label="LibreDash workspaces">
        ${this.renderHeader('', page.title, page.description)}
        <div class="cards">
          ${page.cards?.map((card) => html`
            <article class="card">
              <div>
                <p class="eyebrow">Workspace</p>
                <h2>${card.title}</h2>
                <p class="muted">${card.description}</p>
              </div>
              <footer>
                <span>${card.deploymentLabel}</span>
                <a class="primary-link" href=${card.href}>${lucideIcon(ExternalLink)}<span>Open</span></a>
              </footer>
            </article>
          `)}
        </div>
      </section>
    `
  }

  private renderAssetList(page: WorkspacePageSignal, eyebrow: string, label: string) {
    return html`
      <section class="page" aria-label=${label}>
        ${this.renderHeader(eyebrow, page.title, page.description, this.renderAccessControl())}
        ${renderAssetToolbar(page.assetList?.searchHref ?? '', page.assetList?.query ?? '', page.assetList?.activeType ?? '', page.assetList?.tabs ?? [], 'Search workspace assets...')}
        ${renderAssetTable(page.assetList?.assets ?? [], page.assetList?.empty ?? 'No assets match this view.')}
      </section>
    `
  }

  private renderAccessPage(page: WorkspacePageSignal) {
    return html`
      <section class="page" aria-label="Workspace permissions">
        ${this.renderHeader('Workspace', page.title, page.description, this.renderAccessControl())}
      </section>
    `
  }

  private renderAccessControl() {
    if (!this.workspaceAccess?.canManage) return nothing
    return html`
      <ld-workspace-access-control
        .access=${this.workspaceAccess}
        search=${this.workspaceAccess.search ?? ''}
      ></ld-workspace-access-control>
    `
  }

  private renderHeader(eyebrow: string, title: string, detail = '', actions = nothing) {
    return html`
      <header class="header">
        <div class="title-block">
          ${eyebrow ? html`<p class="eyebrow">${eyebrow}</p>` : nothing}
          <h1>${title}</h1>
          ${detail ? html`<p class="detail">${detail}</p>` : nothing}
        </div>
        <div class="actions">${actions}</div>
      </header>
    `
  }
}

class LibreDashConnectionsPage extends LitElement {
  @property({ converter: jsonAttribute<ConnectionsPageSignal | null>(null) }) page: ConnectionsPageSignal | null = null

  static styles = workspaceStyles

  updated(): void {
    checkSignalContract('connections page', this.page, { kind: 'required', title: 'required', assetList: 'required' })
  }

  render() {
    const page = this.page
    if (!page) return html`<slot></slot>`
    return html`
      <section class="page" aria-label="Connections and sources">
        <header class="header">
          <div class="title-block">
            <p class="eyebrow">Data access</p>
            <h1>${page.title}</h1>
            ${page.description ? html`<p class="detail">${page.description}</p>` : nothing}
          </div>
        </header>
        ${renderAssetToolbar(page.assetList?.searchHref ?? '/connections', page.assetList?.query ?? '', page.assetList?.activeType ?? '', page.assetList?.tabs ?? [], 'Search connections and sources...')}
        ${renderAssetTable(page.assetList?.assets ?? [], page.assetList?.empty ?? 'No connection assets match this view.')}
      </section>
    `
  }
}

class LibreDashWorkspaceAssetPage extends LitElement {
  @property({ converter: jsonAttribute<WorkspaceAssetPageSignal | null>(null) }) page: WorkspaceAssetPageSignal | null = null

  static styles = workspaceStyles

  updated(): void {
    checkSignalContract('workspace asset page', this.page, { title: 'required', breadcrumbs: 'required', tabs: 'required' })
  }

  render() {
    const page = this.page
    if (!page) return html`<slot></slot>`
    return html`
      <section class="asset-page" aria-label="Workspace asset detail">
        <header class="breadcrumb-header">
          <nav aria-label="Breadcrumb">
            <ol>
              ${page.breadcrumbs.map((crumb) => html`
                <li>
                  ${crumb.current
                    ? html`<h1>${assetTypeGlyph(page.asset.type)}<span>${crumb.label}</span></h1>`
                    : html`<a href=${crumb.href}>${crumb.label}</a>`}
                </li>
              `)}
            </ol>
          </nav>
          <div class="actions">
            ${page.actions?.map((action) => html`
              <a class="icon-link" href=${action.href} title=${action.label} aria-label=${action.label}>
                ${action.icon === 'open' ? lucideIcon(ExternalLink) : lucideIcon(ArrowLeft)}
              </a>
            `)}
          </div>
        </header>
        <div class="asset-body">
          ${renderTabs(page.tabs)}
          <div class=${page.activeSection === 'lineage' ? 'section-body lineage-body' : 'section-body'}>
            ${page.activeSection === 'lineage' ? this.renderLineage(page) : this.renderDetails(page)}
          </div>
        </div>
      </section>
    `
  }

  private renderDetails(page: WorkspaceAssetPageSignal) {
    return html`
      <section class="details" id="details" aria-label="Asset details">
        ${renderFacts('Overview', page.details?.overview ?? [], true)}
        ${(page.details?.sections ?? []).map(renderDetailSection)}
      </section>
    `
  }

  private renderLineage(page: WorkspaceAssetPageSignal) {
    return html`
      <section class="lineage" id="lineage" aria-label="Asset lineage">
        <ld-asset-lineage-graph class="lineage-graph" .graph=${page.lineage?.graph ?? { nodes: [], edges: [] }}></ld-asset-lineage-graph>
        <div class="lineage-grids">
          ${renderGridSection('Uses', page.lineage?.usesGrid)}
          ${renderGridSection('Used by', page.lineage?.usedByGrid)}
        </div>
      </section>
    `
  }
}

function renderAssetToolbar(action: string, query: string, activeType: string, tabs: WorkspaceTabSignal[], placeholder: string) {
  return html`
    <div class="toolbar">
      <form method="get" action=${action} class="search">
        <input type="search" name="q" value=${query} placeholder=${placeholder} />
        ${activeType ? html`<input type="hidden" name="type" value=${activeType} />` : nothing}
        <button type="submit" class="icon-button" title="Search" aria-label="Search">${lucideIcon(Search)}</button>
      </form>
      ${renderTabs(tabs)}
    </div>
  `
}

function renderAssetTable(assets: WorkspaceAssetSummarySignal[], empty: string) {
  if (!assets.length) return html`<div class="panel"><div class="empty">${empty}</div></div>`
  return html`
    <div class="panel table-panel">
      <table class="asset-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th class="hide-md">Key</th>
            <th class="hide-lg">Parent</th>
            <th class="right">Actions</th>
          </tr>
        </thead>
        <tbody>
          ${assets.map((asset) => html`
            <tr>
              <td>
                <div class="asset-name">
                  ${assetTypeGlyph(asset.type)}
                  <div>
                    <a class="asset-title" href=${asset.detailHref}>${asset.title}</a>
                    ${asset.description ? html`<p>${asset.description}</p>` : nothing}
                  </div>
                </div>
              </td>
              <td>${asset.typeLabel}</td>
              <td class="hide-md"><code>${asset.key}</code></td>
              <td class="hide-lg">
                ${asset.parentHref
                  ? html`<a class="muted-link" href=${asset.parentHref}>${asset.parentTitle}</a>`
                  : html`<span class="muted">${asset.parentTitle || '-'}</span>`}
              </td>
              <td class="right">
                <span class="row-actions">
                  <a class="icon-link" href=${asset.detailHref} title="View details" aria-label="View details">${lucideIcon(FileText)}</a>
                  <a class="icon-link" href=${asset.openHref} title="Open asset" aria-label="Open asset">${lucideIcon(ExternalLink)}</a>
                </span>
              </td>
            </tr>
          `)}
        </tbody>
      </table>
    </div>
  `
}

function renderTabs(tabs: WorkspaceTabSignal[]) {
  if (!tabs.length) return nothing
  return html`
    <nav class="tabs" aria-label="Asset sections">
      ${tabs.map((tab) => html`
        <a class=${tab.active ? 'active' : ''} href=${tab.href} aria-current=${tab.active ? 'page' : nothing}>
          <span>${tab.label}</span>
          ${tab.count ? html`<span class="count">${tab.count}</span>` : nothing}
        </a>
      `)}
    </nav>
  `
}

function renderDetailSection(section: WorkspaceDetailSectionSignal) {
  if (section.code) {
    return html`
      <section class="detail-section" aria-label=${section.title}>
        <h2>${section.title}</h2>
        <ld-code-block language=${section.lang || 'text'} .code=${section.code}></ld-code-block>
      </section>
    `
  }
  if (section.grid?.columns?.length) return renderGridSection(section.title, section.grid)
  return renderFacts(section.title, section.facts ?? [], false)
}

function renderFacts(title: string, facts: DefinitionFactSignal[], overview: boolean) {
  const filtered = facts.filter((fact) => fact.value?.trim())
  return html`
    <section class="detail-section" aria-label=${title}>
      <h2>${title}</h2>
      ${filtered.length
        ? html`
          <div class=${overview ? 'facts overview' : 'facts'}>
            ${filtered.map((fact) => html`
              <div class=${fact.wide ? 'wide' : ''}>
                <span>${fact.label}</span>
                ${fact.code ? html`<code>${fact.value}</code>` : html`<p>${fact.value}</p>`}
              </div>
            `)}
          </div>
        `
        : html`<div class="empty">No details are available.</div>`}
    </section>
  `
}

function renderGridSection(title: string, grid?: MetricGridSignal) {
  return html`
    <section class="detail-section" aria-label=${title}>
      <h2>${title}</h2>
      <ld-data-grid .grid=${grid ?? null}></ld-data-grid>
    </section>
  `
}

function assetTypeGlyph(type: string) {
  const label = type?.slice(0, 1).toUpperCase() || 'A'
  return html`<span class="asset-glyph" aria-hidden="true">${label}</span>`
}

const workspaceStyles = css`
  :host {
    display: block;
    min-width: 0;
    min-height: 100svh;
    color: var(--ld-fg-default);
    font-family: var(--ld-font-family-ui, var(--fontStack-system));
    background: var(--ld-bg-app);
  }

  .page,
  .asset-page {
    display: grid;
    min-width: 0;
    min-height: 100svh;
    align-content: start;
    gap: var(--base-size-12);
    background: var(--ld-bg-app);
    padding: var(--base-size-16);
  }

  .asset-page {
    grid-template-rows: auto minmax(0, 1fr);
    gap: 0;
    height: 100svh;
    padding: 0;
    overflow: hidden;
  }

  .catalog {
    gap: var(--base-size-16);
  }

  .header,
  .breadcrumb-header {
    display: grid;
    min-width: 0;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--base-size-8);
  }

  .breadcrumb-header {
    border-bottom: var(--ld-border-muted);
    padding: var(--base-size-10) var(--base-size-16);
  }

  .title-block {
    min-width: 0;
  }

  h1,
  h2,
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

  h2 {
    color: var(--ld-fg-default);
    font-size: var(--ld-font-size-body-sm);
    font-weight: var(--ld-font-weight-strong);
  }

  .eyebrow {
    margin-bottom: var(--base-size-4);
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-medium);
    line-height: var(--ld-line-height-tight);
    text-transform: uppercase;
  }

  .detail,
  .muted,
  .asset-table p {
    margin-top: var(--base-size-4);
    overflow: hidden;
    color: var(--ld-fg-muted);
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--ld-font-size-body-sm);
    line-height: var(--ld-line-height-compact);
  }

  .actions,
  .row-actions {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    justify-content: flex-end;
    gap: var(--base-size-8);
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 22rem));
    gap: var(--base-size-16);
    align-items: start;
    justify-content: start;
  }

  .card,
  .panel {
    min-width: 0;
    overflow: hidden;
    border: var(--ld-border-muted);
    border-radius: var(--ld-radius-default);
    background: var(--ld-bg-panel);
  }

  .card {
    display: grid;
    min-height: 10rem;
    grid-template-rows: minmax(0, 1fr) auto;
    padding: var(--base-size-16);
  }

  .card footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--base-size-12);
    margin-top: var(--base-size-16);
    border-top: var(--ld-border-muted);
    padding-top: var(--base-size-12);
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-medium);
  }

  .primary-link,
  .icon-link,
  .icon-button {
    display: inline-grid;
    place-items: center;
    border: var(--ld-border-muted);
    border-radius: var(--ld-radius-default);
    background: var(--ld-bg-panel);
    color: var(--ld-fg-default);
    text-decoration: none;
  }

  .primary-link {
    min-height: var(--control-small-size, 28px);
    grid-auto-flow: column;
    gap: var(--base-size-6);
    background: var(--ld-accent, #0969da);
    color: var(--ld-accent-fg, #fff);
    padding: 0 var(--base-size-10);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-strong);
  }

  .icon-link,
  .icon-button {
    width: var(--control-medium-size);
    height: var(--control-medium-size);
    padding: 0;
  }

  button,
  input {
    font: inherit;
  }

  .toolbar {
    display: grid;
    min-width: 0;
    gap: var(--base-size-12);
    border-bottom: var(--ld-border-default);
    padding-top: var(--base-size-12);
  }

  .search {
    display: flex;
    max-width: 34rem;
    min-width: 0;
    gap: var(--base-size-8);
  }

  input[type='search'] {
    min-width: 0;
    min-height: var(--control-medium-size);
    width: 100%;
    border: var(--ld-border-default);
    border-radius: var(--ld-radius-tight);
    background: var(--ld-bg-control);
    color: var(--ld-fg-default);
    padding: 0 var(--base-size-12);
  }

  .tabs {
    display: flex;
    min-width: 0;
    flex-wrap: wrap;
    gap: var(--base-size-24);
    border-bottom: var(--ld-border-default);
  }

  .toolbar .tabs {
    border-bottom: 0;
  }

  .tabs a {
    display: inline-flex;
    min-height: var(--control-xlarge-size);
    align-items: center;
    gap: var(--base-size-8);
    border-bottom: 2px solid transparent;
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-body-sm);
    font-weight: var(--ld-font-weight-medium);
    text-decoration: none;
  }

  .tabs a.active {
    border-bottom-color: var(--ld-accent, #0969da);
    color: var(--ld-fg-default);
    font-weight: var(--ld-font-weight-strong);
  }

  .count {
    display: inline-grid;
    min-width: var(--base-size-16);
    place-items: center;
    border-radius: var(--ld-radius-full);
    background: var(--ld-bg-panel-muted);
    color: var(--ld-fg-muted);
    padding: 0 var(--base-size-6);
    font-size: var(--ld-font-size-caption);
  }

  .table-panel {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    table-layout: fixed;
  }

  th,
  td {
    border-bottom: var(--ld-border-muted);
    padding: var(--base-size-8) var(--base-size-12);
    vertical-align: middle;
  }

  th {
    background: var(--ld-bg-panel-muted);
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-medium);
    text-transform: uppercase;
  }

  td {
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-body-sm);
    font-weight: var(--ld-font-weight-medium);
  }

  tr:hover td {
    background: var(--ld-bg-control-hover);
  }

  .right {
    text-align: right;
  }

  .asset-name {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: var(--base-size-12);
  }

  .asset-title,
  .muted-link {
    color: var(--ld-fg-default);
    text-decoration: none;
  }

  .muted-link {
    color: var(--ld-fg-link);
  }

  code {
    color: var(--ld-fg-muted);
    font-family: var(--fontStack-monospace, ui-monospace, SFMono-Regular, Consolas, monospace);
    font-size: var(--ld-font-size-caption);
  }

  .asset-glyph {
    display: inline-grid;
    width: var(--control-medium-size);
    height: var(--control-medium-size);
    flex: 0 0 auto;
    place-items: center;
    border: var(--ld-border-muted);
    border-radius: var(--ld-radius-default);
    background: var(--ld-bg-panel-muted);
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-strong);
  }

  .empty {
    color: var(--ld-fg-muted);
    padding: var(--base-size-12);
    font-size: var(--ld-font-size-body-sm);
  }

  .breadcrumb-header ol {
    display: flex;
    min-width: 0;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--base-size-6);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--ld-font-size-body-sm);
    font-weight: var(--ld-font-weight-medium);
  }

  .breadcrumb-header li:not(:last-child)::after {
    content: '/';
    margin-left: var(--base-size-6);
    color: var(--ld-fg-muted);
  }

  .breadcrumb-header a {
    color: var(--ld-fg-muted);
    text-decoration: none;
  }

  .breadcrumb-header h1 {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    gap: var(--base-size-8);
  }

  .asset-body {
    display: grid;
    min-width: 0;
    min-height: 0;
    grid-template-rows: auto minmax(0, 1fr);
  }

  .section-body {
    min-height: 0;
    overflow: auto;
    padding: var(--base-size-16);
  }

  .lineage-body {
    padding: 0;
  }

  .details,
  .lineage-grids {
    display: grid;
    align-content: start;
    gap: var(--base-size-24);
  }

  .lineage {
    display: grid;
    min-height: 0;
    align-content: start;
  }

  .lineage-graph {
    display: block;
    height: var(--ld-lineage-graph-height, 32rem);
    min-height: 0;
    border-bottom: var(--ld-border-muted);
    background: var(--ld-bg-panel);
  }

  .lineage-grids {
    padding: var(--base-size-16);
  }

  .detail-section {
    display: grid;
    min-width: 0;
    align-content: start;
    gap: var(--base-size-12);
    border-bottom: var(--ld-border-muted);
    padding-bottom: var(--base-size-20);
  }

  .detail-section:last-child {
    border-bottom: 0;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: var(--base-size-12) var(--base-size-20);
  }

  .facts.overview {
    grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  }

  .facts .wide {
    grid-column: span 2;
  }

  .facts div {
    display: grid;
    min-width: 0;
    gap: var(--base-size-4);
  }

  .facts span:first-child {
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-medium);
    text-transform: uppercase;
  }

  .facts p,
  .facts code {
    overflow: hidden;
    color: var(--ld-fg-default);
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--ld-font-size-body-sm);
  }

  .facts .wide p,
  .facts .wide code {
    white-space: pre-wrap;
  }

  @media (max-width: 720px) {
    .page {
      padding: var(--base-size-12);
    }

    .header,
    .breadcrumb-header {
      grid-template-columns: 1fr;
    }

    .asset-page {
      height: auto;
      min-height: 100svh;
      overflow: visible;
    }

    .hide-md,
    .hide-lg {
      display: none;
    }

    .section-body {
      overflow: visible;
    }
  }
`

if (!customElements.get('ld-workspace-page')) customElements.define('ld-workspace-page', LibreDashWorkspacePage)
if (!customElements.get('ld-workspace-asset-page')) customElements.define('ld-workspace-asset-page', LibreDashWorkspaceAssetPage)
if (!customElements.get('ld-connections-page')) customElements.define('ld-connections-page', LibreDashConnectionsPage)
