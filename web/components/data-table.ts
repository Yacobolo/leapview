import { LitElement, css, html, nothing } from 'lit'
import { createRef, ref, type Ref } from 'lit/directives/ref.js'
import { visualMenuIcon } from './visual-menu-icons'

type SortDirection = 'asc' | 'desc'
type BlockID = 'a' | 'b' | 'c'

interface TableSort {
  key: string
  direction: SortDirection
}

interface TableColumn {
  key: string
  label: string
  align?: 'left' | 'right'
}

type TableRow = Record<string, unknown>

interface TableBlock {
  start: number
  rows: TableRow[]
}

interface TableSignal {
  version: number
  title: string
  columns: TableColumn[]
  totalRows: number
  availableRows: number
  isCapped: boolean
  rowCap: number
  chunkSize: number
  rowHeight: number
  resetVersion: number
  sort: TableSort
  blocks: Record<BlockID, TableBlock>
  loadingBlock: string
  error: string
}

interface TableBlockCommand {
  table: string
  block: BlockID | 'all'
  start: number
  count: number
  sort: TableSort
  resetVersion: number
}

type VisualAction = 'focus' | 'show-data' | 'copy-data' | 'export-csv' | 'clear-selection'

const blockIDs: BlockID[] = ['a', 'b', 'c']
const defaultChunkSize = 200
const defaultRowHeight = 34
const defaultSort: TableSort = { key: 'purchase_date', direction: 'desc' }

const emptyTable: TableSignal = {
  version: 2,
  title: 'Orders',
  columns: [],
  totalRows: 0,
  availableRows: 0,
  isCapped: false,
  rowCap: 10000,
  chunkSize: defaultChunkSize,
  rowHeight: defaultRowHeight,
  resetVersion: 0,
  sort: defaultSort,
  blocks: emptyBlocks(),
  loadingBlock: '',
  error: '',
}

const tableConverter = {
  fromAttribute(value: string | null): TableSignal {
    if (!value) return emptyTable
    try {
      return normalizeTable(JSON.parse(value) as Partial<TableSignal>)
    } catch {
      return { ...emptyTable, error: 'Could not parse table signal.' }
    }
  },
  toAttribute(value: TableSignal | null): string {
    return JSON.stringify(value ?? emptyTable)
  },
}

function emptyBlocks(): Record<BlockID, TableBlock> {
  return {
    a: { start: 0, rows: [] },
    b: { start: defaultChunkSize, rows: [] },
    c: { start: defaultChunkSize * 2, rows: [] },
  }
}

function normalizeTable(value: Partial<TableSignal>): TableSignal {
  const chunkSize = positiveNumber(value.chunkSize, defaultChunkSize)
  return {
    ...emptyTable,
    ...value,
    version: 2,
    totalRows: positiveNumber(value.totalRows, 0),
    availableRows: positiveNumber(value.availableRows, positiveNumber(value.totalRows, 0)),
    rowCap: positiveNumber(value.rowCap, 10000),
    chunkSize,
    rowHeight: positiveNumber(value.rowHeight, defaultRowHeight),
    resetVersion: positiveNumber(value.resetVersion, 0),
    sort: value.sort?.key ? value.sort : defaultSort,
    columns: Array.isArray(value.columns) ? value.columns : [],
    blocks: {
      a: normalizeBlock(value.blocks?.a, 0),
      b: normalizeBlock(value.blocks?.b, chunkSize),
      c: normalizeBlock(value.blocks?.c, chunkSize * 2),
    },
    loadingBlock: value.loadingBlock ?? '',
    error: value.error ?? '',
  }
}

function normalizeBlock(block: TableBlock | undefined, fallbackStart: number): TableBlock {
  return {
    start: positiveNumber(block?.start, fallbackStart),
    rows: Array.isArray(block?.rows) ? block.rows : [],
  }
}

function positiveNumber(value: unknown, fallback: number): number {
  const next = Number(value)
  return Number.isFinite(next) && next >= 0 ? next : fallback
}

function formatCell(value: unknown, column: TableColumn): string {
  if (value === null || value === undefined || value === '') return '-'
  if (column.key === 'revenue' && Number.isFinite(Number(value))) {
    return `R$ ${Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })}`
  }
  if (column.key === 'review_score' && Number.isFinite(Number(value))) {
    return Number(value).toFixed(2)
  }
  if (column.key === 'delivery_days' && Number.isFinite(Number(value))) {
    return `${Number(value)}d`
  }
  return String(value)
}

function defaultDirection(column: TableColumn): SortDirection {
  return ['revenue', 'review_score', 'delivery_days', 'purchase_date'].includes(column.key) ? 'desc' : 'asc'
}

function rowKey(row: TableRow, fallback: number): string {
  const id = row.order_id
  return typeof id === 'string' && id ? id : String(fallback)
}

class DataTable extends LitElement {
  static properties = {
    tableId: { attribute: 'table-id' },
    table: { attribute: 'table', converter: tableConverter },
    selectedRowId: { state: true },
    selectedCellKey: { state: true },
    viewportTop: { state: true },
    viewportHeight: { state: true },
  }

  tableId = 'orders'
  table: TableSignal = emptyTable
  private selectedRowId = ''
  private selectedCellKey = ''
  private viewportTop = 0
  private viewportHeight = 0
  private lastResetVersion = -1
  private shouldResetScroll = false
  private pendingLoads = new Set<string>()
  private blockCache: Record<BlockID, TableBlock> = emptyBlocks()
  private scrollElementRef: Ref<HTMLDivElement> = createRef()
  private resizeObserver?: ResizeObserver
  private handleOutsidePointerDown = (event: PointerEvent) => {
    const details = this.renderRoot.querySelector<HTMLDetailsElement>('.visual-options')
    if (!details?.open) return
    if (!event.composedPath().includes(details)) details.removeAttribute('open')
  }
  private handleDocumentKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return
    this.renderRoot.querySelector<HTMLDetailsElement>('.visual-options')?.removeAttribute('open')
  }

  static styles = css`
    :host {
      display: block;
      height: 100%;
      min-height: 0;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    .shell {
      display: flex;
      flex-direction: column;
      height: 100%;
      min-height: 0;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
    }

    .toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      min-height: 34px;
      border-bottom: 1px solid var(--borderColor-default);
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      padding: 6px 8px 5px 10px;
    }

    h2 {
      min-width: 0;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.8rem;
      font-weight: 850;
      letter-spacing: 0;
      line-height: 1.1;
    }

    .visual-options {
      position: relative;
      flex: 0 0 auto;
    }

    .visual-options summary {
      display: grid;
      width: 24px;
      height: 24px;
      place-items: center;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: var(--fgColor-muted);
      cursor: pointer;
      font-size: 1rem;
      font-weight: 900;
      line-height: 1;
      list-style: none;
    }

    .visual-options summary::-webkit-details-marker {
      display: none;
    }

    .visual-options summary:hover,
    .visual-options summary:focus-visible,
    .visual-options[open] summary {
      border-color: var(--borderColor-default);
      background: var(--bgColor-muted);
      color: var(--fgColor-default);
      outline: 0;
    }

    .menu {
      position: absolute;
      top: calc(100% + 4px);
      right: 0;
      z-index: 30;
      display: grid;
      width: 176px;
      border: 1px solid var(--borderColor-default);
      border-radius: 6px;
      background: var(--overlay-bgColor, var(--bgColor-default));
      box-shadow: var(--shadow-floating-small, 0 8px 24px rgb(0 0 0 / 18%));
      padding: 4px;
    }

    .menu button {
      display: flex;
      align-items: center;
      gap: 8px;
      min-height: 27px;
      border: 0;
      border-radius: 4px;
      background: transparent;
      color: var(--fgColor-default);
      cursor: pointer;
      padding: 0 8px;
      font: inherit;
      font-size: 0.68rem;
      font-weight: 750;
      text-align: left;
    }

    .menu svg {
      flex: 0 0 auto;
      width: 14px;
      height: 14px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 2;
    }

    .menu button:hover,
    .menu button:focus-visible {
      background: var(--bgColor-muted);
      outline: 0;
    }

    .menu button:disabled {
      cursor: default;
      opacity: 0.48;
    }

    .menu button:disabled:hover {
      background: transparent;
    }

    .error {
      border-bottom: 1px solid var(--borderColor-danger-emphasis);
      background: var(--bgColor-danger-muted);
      color: var(--fgColor-danger);
      padding: 9px 12px;
      font-size: 0.82rem;
      font-weight: 850;
    }

    .head,
    .row {
      display: grid;
      grid-template-columns: var(--ld-table-columns);
      min-width: 1080px;
    }

    .head {
      position: relative;
      z-index: 1;
      border-bottom: 1px solid var(--borderColor-emphasis);
      background: var(--bgColor-muted);
      color: var(--fgColor-muted);
      box-shadow: inset 0 -1px 0 var(--borderColor-emphasis);
    }

    .header-cell,
    .cell {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .header-cell {
      border-right: 1px solid var(--borderColor-default);
    }

    .header-cell:last-child {
      border-right: 0;
    }

    button.header-button {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      width: 100%;
      min-height: 34px;
      border: 0;
      border-bottom: 2px solid transparent;
      background: transparent;
      color: inherit;
      cursor: pointer;
      padding: 0 9px;
      font: inherit;
      font-size: 0.7rem;
      font-weight: 900;
      letter-spacing: 0;
      text-align: left;
      text-transform: uppercase;
    }

    button.header-button:hover,
    button.header-button:focus-visible,
    .sorted button.header-button {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 92%);
      color: var(--fgColor-default);
      outline: 0;
    }

    .sorted button.header-button {
      border-bottom-color: var(--fgColor-accent);
    }

    .sort {
      display: inline-grid;
      min-width: 18px;
      place-items: center;
      color: var(--fgColor-accent);
      font-size: 0.82rem;
      opacity: 0;
    }

    .sorted .sort {
      opacity: 1;
    }

    .viewport {
      position: relative;
      flex: 1 1 auto;
      overflow: auto;
      min-height: 0;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      scrollbar-gutter: stable;
    }

    .canvas {
      position: relative;
      min-width: 1080px;
    }

    .row {
      position: absolute;
      inset-inline: 0;
      height: var(--ld-row-height, 34px);
      border-bottom: 1px solid var(--borderColor-muted);
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      color: var(--fgColor-default);
    }

    .row:nth-child(even) {
      background: color-mix(in srgb, var(--report-table-stripe, var(--bgColor-muted)), var(--report-chart-surface, var(--bgColor-default)) 45%);
    }

    .row:hover {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 91%);
    }

    .row.selected {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 86%);
      box-shadow: inset 3px 0 0 var(--fgColor-accent);
    }

    .cell {
      display: flex;
      align-items: center;
      min-width: 0;
      border: 0;
      border-right: 1px solid var(--borderColor-muted);
      background: transparent;
      color: inherit;
      cursor: default;
      font: inherit;
      padding: 0 9px;
      font-size: 0.77rem;
      font-weight: 600;
      text-align: left;
    }

    .cell:last-child {
      border-right: 0;
    }

    .cell.active {
      outline: 2px solid var(--fgColor-accent);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--fgColor-accent), transparent 88%);
    }

    .right {
      justify-content: end;
      font-variant-numeric: tabular-nums;
    }

    .empty {
      display: grid;
      min-height: 240px;
      place-items: center;
      color: var(--fgColor-muted);
      font-size: 0.9rem;
      font-weight: 850;
    }

    .loading {
      position: absolute;
      inset-inline: 0;
      top: 0;
      z-index: 2;
      height: 3px;
      overflow: hidden;
      background: var(--bgColor-accent-muted);
    }

    .loading::after {
      content: '';
      display: block;
      width: 34%;
      height: 100%;
      background: var(--fgColor-accent);
      animation: load 900ms ease-in-out infinite;
    }

    .footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      min-height: 34px;
      border-top: 1px solid var(--borderColor-default);
      background: var(--report-panel-subtle, var(--bgColor-muted));
      padding: 6px 10px;
      color: var(--fgColor-muted);
      font-size: 0.72rem;
      font-weight: 750;
    }

    .footer strong {
      color: var(--fgColor-default);
      font-weight: 850;
    }

    @keyframes load {
      0% { transform: translateX(-100%); }
      100% { transform: translateX(310%); }
    }

    @media (max-width: 760px) {
      .shell {
        min-height: 360px;
      }

      .toolbar,
      .footer {
        align-items: stretch;
        flex-direction: column;
      }
    }
  `

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('pointerdown', this.handleOutsidePointerDown)
    document.addEventListener('keydown', this.handleDocumentKeyDown)
  }

  firstUpdated(): void {
    const viewport = this.scrollElementRef.value
    if (!viewport) return
    this.viewportHeight = viewport.clientHeight
    this.resizeObserver = new ResizeObserver(() => {
      this.viewportHeight = viewport.clientHeight
      this.ensureBlocksForScroll()
    })
    this.resizeObserver.observe(viewport)
    this.ensureBlocksForScroll()
  }

  disconnectedCallback(): void {
    document.removeEventListener('pointerdown', this.handleOutsidePointerDown)
    document.removeEventListener('keydown', this.handleDocumentKeyDown)
    this.resizeObserver?.disconnect()
    super.disconnectedCallback()
  }

  willUpdate(): void {
    if (this.lastResetVersion !== this.table.resetVersion) {
      this.lastResetVersion = this.table.resetVersion
      this.blockCache = emptyBlocks()
      this.shouldResetScroll = true
      this.pendingLoads.clear()
      this.selectedRowId = ''
      this.selectedCellKey = ''
    }
    this.mergeIncomingBlocks()
    this.clearArrivedLoads()
    if (this.selectedRowId && !this.loadedRows.some((item) => rowKey(item.row, item.index) === this.selectedRowId)) {
      this.selectedRowId = ''
      this.selectedCellKey = ''
    }
  }

  updated(): void {
    if (this.shouldResetScroll) {
      this.shouldResetScroll = false
      queueMicrotask(() => {
        const viewport = this.scrollElementRef.value
        if (!viewport) return
        viewport.scrollTop = 0
        this.viewportTop = 0
        this.viewportHeight = viewport.clientHeight
      })
    }
  }

  get columns(): TableColumn[] {
    return Array.isArray(this.table?.columns) ? this.table.columns : []
  }

  get loadedRows(): Array<{ row: TableRow; index: number }> {
    return blockIDs
      .map((id) => this.blocks[id])
      .sort((a, b) => a.start - b.start)
      .flatMap((block) => block.rows.map((row, offset) => ({ row, index: block.start + offset })))
      .filter((item) => item.index < this.availableRows)
  }

  get availableRows(): number {
    return Math.max(0, this.table.availableRows ?? 0)
  }

  get blocks(): Record<BlockID, TableBlock> {
    return this.blockCache
  }

  get chunkSize(): number {
    return Math.max(1, this.table.chunkSize || defaultChunkSize)
  }

  get rowHeight(): number {
    return Math.max(1, this.table.rowHeight || defaultRowHeight)
  }

  get gridTemplate(): string {
    const widths: Record<string, string> = {
      order_id: 'minmax(210px,1.35fr)',
      purchase_date: 'minmax(118px,.75fr)',
      status: 'minmax(118px,.75fr)',
      state: 'minmax(70px,.42fr)',
      category: 'minmax(190px,1.1fr)',
      revenue: 'minmax(120px,.72fr)',
      review_score: 'minmax(96px,.55fr)',
      delivery_days: 'minmax(96px,.55fr)',
    }
    return this.columns.map((column) => widths[column.key] ?? 'minmax(120px,1fr)').join(' ')
  }

  handleScroll(event: Event): void {
    const target = event.currentTarget as HTMLDivElement
    this.viewportTop = target.scrollTop
    this.viewportHeight = target.clientHeight
    this.ensureBlocksForScroll()
  }

  sortColumn(column: TableColumn): void {
    const current = this.table?.sort ?? defaultSort
    const direction: SortDirection = current.key === column.key
      ? current.direction === 'asc' ? 'desc' : 'asc'
      : defaultDirection(column)
    this.emitBlock('all', 0, { key: column.key, direction }, this.table.resetVersion + 1)
  }

  selectCell(row: TableRow, column: TableColumn, absoluteIndex: number): void {
    const key = rowKey(row, absoluteIndex)
    this.selectedRowId = key
    this.selectedCellKey = `${key}:${column.key}`
  }

  render() {
    const columns = this.columns
    const loadedRows = this.loadedRows
    const totalHeight = this.availableRows * this.rowHeight
    const rowRange = this.rowRangeText()
    const selectedText = this.selectedRowId ? '1 row selected' : 'No selection'
    const loading = Boolean(this.table.loadingBlock)

    return html`
      <section class="shell" style=${`--ld-table-columns:${this.gridTemplate};--ld-row-height:${this.rowHeight}px`}>
        <div class="toolbar">
          <div>
            <h2>${this.table?.title ?? 'Orders'}</h2>
          </div>
          <details class="visual-options">
            <summary aria-label="Visual options" title="Visual options">⋮</summary>
            <div class="menu" role="menu">
              <button type="button" role="menuitem" @click=${() => this.runAction('focus')}>${visualMenuIcon('focus')}<span>Focus mode</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction('show-data')}>${visualMenuIcon('show-data')}<span>Show data</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction('copy-data')}>${visualMenuIcon('copy-data')}<span>Copy data</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction('export-csv')}>${visualMenuIcon('export-csv')}<span>Export CSV</span></button>
              <button type="button" role="menuitem" ?disabled=${!this.selectedRowId} @click=${() => this.runAction('clear-selection')}>${visualMenuIcon('clear-selection')}<span>Clear selection</span></button>
            </div>
          </details>
        </div>
        ${this.table?.error ? html`<div class="error">${this.table.error}</div>` : nothing}
        <div class="head" role="row">
          ${columns.map((column) => {
            const sorted = this.table?.sort?.key === column.key
            const sortMark = this.table?.sort?.direction === 'asc' ? '↑' : '↓'
            return html`
              <div class=${`header-cell ${sorted ? 'sorted' : ''}`} role="columnheader">
                <button class="header-button" type="button" @click=${() => this.sortColumn(column)}>
                  <span>${column.label}</span>
                  <span class="sort">${sortMark}</span>
                </button>
              </div>
            `
          })}
        </div>
        <div class="viewport" ${ref(this.scrollElementRef)} @scroll=${this.handleScroll} role="table" aria-label=${this.table?.title ?? 'Orders'}>
          ${loading ? html`<div class="loading" aria-hidden="true"></div>` : nothing}
          ${this.availableRows === 0 && !loading ? html`<div class="empty">Waiting for table data</div>` : html`
            <div class="canvas" style=${`height:${totalHeight}px`}>
              ${loadedRows.map(({ row, index }) => {
                const key = rowKey(row, index)
                const selected = key === this.selectedRowId
                return html`
                  <div
                    class=${`row ${selected ? 'selected' : ''}`}
                    role="row"
                    aria-selected=${selected ? 'true' : 'false'}
                    style=${`transform:translateY(${index * this.rowHeight}px)`}
                    @click=${() => {
                      this.selectedRowId = key
                      this.selectedCellKey = ''
                    }}
                  >
                    ${columns.map((column) => {
                      const cellKey = `${key}:${column.key}`
                      return html`
                        <button
                          class=${`cell ${column.align === 'right' ? 'right' : ''} ${cellKey === this.selectedCellKey ? 'active' : ''}`}
                          role="cell"
                          title=${String(row[column.key] ?? '')}
                          @click=${(event: Event) => {
                            event.stopPropagation()
                            this.selectCell(row, column, index)
                          }}
                        >
                          ${formatCell(row[column.key], column)}
                        </button>
                      `
                    })}
                  </div>
                `
              })}
            </div>
          `}
        </div>
        <div class="footer">
          <span><strong>${rowRange}</strong>${this.table.isCapped ? html` · browsing first ${this.table.rowCap.toLocaleString()}` : nothing}</span>
          <span>${selectedText}</span>
        </div>
      </section>
    `
  }

  private ensureBlocksForScroll(): void {
    if (this.availableRows <= 0) return
    const currentStart = Math.floor(Math.floor(this.viewportTop / this.rowHeight) / this.chunkSize) * this.chunkSize
    const desired = this.desiredStarts(currentStart)
    const desiredSet = new Set(desired)
    const loadedStarts = new Set(blockIDs.map((id) => this.blocks[id]?.start ?? -1))
    const pendingStarts = new Set([...this.pendingLoads].map((key) => Number(key.split(':')[1])))
    const usedBlocks = new Set<BlockID>()

    for (const start of desired) {
      if (loadedStarts.has(start) || pendingStarts.has(start)) continue
      const block = this.reusableBlock(desiredSet, usedBlocks)
      if (!block) continue
      usedBlocks.add(block)
      this.emitBlock(block, start, this.table.sort, this.table.resetVersion)
    }
  }

  private desiredStarts(currentStart: number): number[] {
    const starts = currentStart <= 0
      ? [0, this.chunkSize, this.chunkSize * 2]
      : [Math.max(0, currentStart - this.chunkSize), currentStart, currentStart + this.chunkSize]
    return starts.filter((start, index, all) => start < this.availableRows && all.indexOf(start) === index)
  }

  private reusableBlock(desiredStarts: Set<number>, usedBlocks: Set<BlockID>): BlockID | undefined {
    return blockIDs.find((id) => !usedBlocks.has(id) && !desiredStarts.has(this.blocks[id]?.start ?? -1))
      ?? blockIDs.find((id) => !usedBlocks.has(id))
  }

  private emitBlock(block: BlockID | 'all', start: number, sort = this.table.sort, resetVersion = this.table.resetVersion): void {
    const count = this.chunkSize
    const key = `${block}:${start}:${sort.key}:${sort.direction}:${resetVersion}`
    if (this.pendingLoads.has(key)) return
    this.pendingLoads.add(key)
    this.dispatchEvent(new CustomEvent<TableBlockCommand>('ld-table-window-change', {
      bubbles: true,
      composed: true,
      detail: {
        table: this.tableId || 'orders',
        block,
        start,
        count,
        sort,
        resetVersion,
      },
    }))
  }

  private clearArrivedLoads(): void {
    for (const key of [...this.pendingLoads]) {
      const [block, start, sortKey, sortDirection, resetVersion] = key.split(':')
      if (block === 'all') {
        if (this.table.resetVersion === Number(resetVersion) && this.table.sort.key === sortKey && this.table.sort.direction === sortDirection) {
          this.pendingLoads.delete(key)
        }
        continue
      }
      const tableBlock = this.blocks[block as BlockID]
      if (
        tableBlock?.start === Number(start)
        && this.table.sort.key === sortKey
        && this.table.sort.direction === sortDirection
        && this.table.resetVersion === Number(resetVersion)
      ) {
        this.pendingLoads.delete(key)
      }
    }
  }

  private rowRangeText(): string {
    if (!this.table.totalRows || !this.availableRows) return 'No rows'
    const firstIndex = Math.min(this.availableRows - 1, Math.max(0, Math.floor(this.viewportTop / this.rowHeight)))
    const visibleRows = Math.max(1, Math.ceil((this.viewportHeight || this.rowHeight) / this.rowHeight))
    const lastIndex = Math.min(this.availableRows, firstIndex + visibleRows)
    return `${(firstIndex + 1).toLocaleString()}-${lastIndex.toLocaleString()} of ${this.table.totalRows.toLocaleString()}`
  }

  private mergeIncomingBlocks(): void {
    const defaults = emptyBlocks()
    for (const id of blockIDs) {
      const incoming = this.table.blocks[id]
      if (!incoming) continue
      const defaultBlock = defaults[id]
      const carriesRows = incoming.rows.length > 0
      const carriesNonDefaultStart = incoming.start !== defaultBlock.start
      const cacheIsEmpty = this.blockCache[id].rows.length === 0
      if (carriesRows || carriesNonDefaultStart || cacheIsEmpty) {
        this.blockCache[id] = { start: incoming.start, rows: incoming.rows }
      }
    }
  }

  private runAction(action: VisualAction): void {
    this.renderRoot.querySelector<HTMLDetailsElement>('.visual-options')?.removeAttribute('open')
    if (action === 'clear-selection') {
      this.selectedRowId = ''
      this.selectedCellKey = ''
    }
    this.dispatchEvent(
      new CustomEvent('ld-visual-action', {
        bubbles: true,
        composed: true,
        detail: {
          action,
          visualType: 'table',
          visualId: this.tableId || 'orders',
          title: this.table?.title ?? 'Orders',
          columns: this.columns,
          rows: this.exportRows(),
          selection: this.selectedRowId ? [this.selectedRowId] : [],
          table: {
            ...(this.table ?? emptyTable),
            blocks: this.blocks,
            rows: this.exportRows(),
            columns: this.columns,
          },
        },
      }),
    )
  }

  private exportRows(): TableRow[] {
    return this.loadedRows.map(({ row }) => {
      const next: TableRow = {}
      for (const column of this.columns) {
        next[column.key] = formatCell(row[column.key], column)
      }
      return next
    })
  }
}

customElements.define('ld-data-table', DataTable)
