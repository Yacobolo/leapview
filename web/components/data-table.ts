import { LitElement, css, html, nothing } from 'lit'
import { createRef, ref, type Ref } from 'lit/directives/ref.js'
import {
  flexRender,
  getCoreRowModel,
  TableController,
  type ColumnDef,
} from '@tanstack/lit-table'
import { VirtualizerController } from '@tanstack/lit-virtual'
import { visualMenuIcon } from './visual-menu-icons'

type SortDirection = 'asc' | 'desc'

interface TableSort {
  key: string
  direction: SortDirection
}

interface TableWindow {
  offset: number
  limit: number
}

interface TableColumn {
  key: string
  label: string
  align?: 'left' | 'right'
}

type TableRow = Record<string, unknown>

interface TableSignal {
  title: string
  columns: TableColumn[]
  rows: TableRow[]
  totalRows: number
  window: TableWindow
  sort: TableSort
  loading: boolean
  error: string
}

interface TableWindowCommand {
  table: string
  offset: number
  limit: number
  sort: TableSort
}

type VisualAction = 'focus' | 'show-data' | 'copy-data' | 'export-csv' | 'clear-selection'

const emptyTable: TableSignal = {
  title: 'Orders',
  columns: [],
  rows: [],
  totalRows: 0,
  window: { offset: 0, limit: 120 },
  sort: { key: 'purchase_date', direction: 'desc' },
  loading: false,
  error: '',
}

const tableConverter = {
  fromAttribute(value: string | null): TableSignal {
    if (!value) return emptyTable
    try {
      return { ...emptyTable, ...JSON.parse(value) } as TableSignal
    } catch {
      return { ...emptyTable, error: 'Could not parse table signal.' }
    }
  },
  toAttribute(value: TableSignal | null): string {
    return JSON.stringify(value ?? emptyTable)
  },
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
  }

  tableId = 'orders'
  table: TableSignal = emptyTable
  private selectedRowId = ''
  private selectedCellKey = ''
  private pendingKey = ''
  private scrollElementRef: Ref<HTMLDivElement> = createRef()
  private tableController = new TableController<TableRow>(this)
  private virtualizerController = new VirtualizerController<HTMLDivElement, Element>(this, {
    getScrollElement: () => this.scrollElementRef.value,
    count: 0,
    estimateSize: () => 34,
    overscan: 10,
  })

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

    .eyebrow {
      margin: 0 0 3px;
      color: var(--fgColor-muted);
      font-size: 0.68rem;
      font-weight: 900;
      letter-spacing: 0;
      text-transform: uppercase;
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

    .footer {
      display: flex;
      align-items: center;
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
      height: 34px;
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

      .visual-actions {
        align-self: end;
      }
    }
  `

  updated(): void {
    const key = this.requestKey(this.table?.window?.offset, this.table?.sort)
    if (!this.table?.loading && this.pendingKey === key) {
      this.pendingKey = ''
    }
    if (this.selectedRowId && !this.rows.some((row, index) => rowKey(row, index) === this.selectedRowId)) {
      this.selectedRowId = ''
      this.selectedCellKey = ''
    }
  }

  get rows(): TableRow[] {
    return Array.isArray(this.table?.rows) ? this.table.rows : []
  }

  get columns(): TableColumn[] {
    return Array.isArray(this.table?.columns) ? this.table.columns : []
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

  requestKey(offset: number | undefined, sort = this.table?.sort): string {
    return `${offset ?? 0}:${this.table?.window?.limit ?? 120}:${sort?.key ?? ''}:${sort?.direction ?? ''}`
  }

  emitWindow(offset: number, sort = this.table?.sort): void {
    const limit = this.table?.window?.limit ?? 120
    const maxOffset = Math.max(0, (this.table?.totalRows ?? 0) - limit)
    const nextOffset = Math.max(0, Math.min(offset, maxOffset))
    const nextSort = sort?.key ? sort : { key: 'purchase_date', direction: 'desc' as SortDirection }
    const key = this.requestKey(nextOffset, nextSort)
    if (this.pendingKey === key || this.table?.loading) return

    this.pendingKey = key
    this.dispatchEvent(new CustomEvent<TableWindowCommand>('ld-table-window-change', {
      bubbles: true,
      composed: true,
      detail: {
        table: 'orders',
        offset: nextOffset,
        limit,
        sort: nextSort,
      },
    }))
  }

  handleScroll(event: Event): void {
    const target = event.currentTarget as HTMLDivElement
    const offset = this.table?.window?.offset ?? 0
    const limit = this.table?.window?.limit ?? 120
    const total = this.table?.totalRows ?? 0
    const nearBottom = target.scrollTop + target.clientHeight > target.scrollHeight - 160
    const nearTop = target.scrollTop < 80

    if (nearBottom && offset + limit < total) {
      this.emitWindow(offset + limit)
    } else if (nearTop && offset > 0) {
      this.emitWindow(offset - limit)
    }
  }

  sortColumn(column: TableColumn): void {
    const current = this.table?.sort ?? {}
    const direction: SortDirection = current.key === column.key
      ? current.direction === 'asc' ? 'desc' : 'asc'
      : defaultDirection(column)
    this.emitWindow(0, { key: column.key, direction })
  }

  selectCell(row: TableRow, column: TableColumn, rowIndex: number): void {
    const key = rowKey(row, rowIndex)
    this.selectedRowId = key
    this.selectedCellKey = `${key}:${column.key}`
  }

  render() {
    const rows = this.rows
    const columns = this.columns
    const table = this.tableController.table({
      data: rows,
      columns: columns.map((column): ColumnDef<TableRow> => ({
        id: column.key,
        accessorKey: column.key,
        header: () => column.label,
        cell: (info) => formatCell(info.getValue(), column),
      })),
      getCoreRowModel: getCoreRowModel(),
      renderFallbackValue: '-',
      manualSorting: true,
    })

    const rowModel = table.getRowModel().rows
    const virtualizer = this.virtualizerController.getVirtualizer()
    virtualizer.setOptions({
      ...virtualizer.options,
      count: rowModel.length,
      estimateSize: () => 34,
      overscan: 10,
    })
    const virtualRows = virtualizer.getVirtualItems()
    const totalSize = virtualizer.getTotalSize()
    const first = (this.table?.window?.offset ?? 0) + 1
    const last = Math.min((this.table?.window?.offset ?? 0) + rows.length, this.table?.totalRows ?? 0)
    const rowRange = this.table?.totalRows ? `${first.toLocaleString()}-${last.toLocaleString()} of ${this.table.totalRows.toLocaleString()}` : 'No rows'
    const selectedText = this.selectedRowId ? '1 row selected' : 'No selection'

    return html`
      <section class="shell" style=${`--ld-table-columns:${this.gridTemplate}`}>
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
          ${this.table?.loading ? html`<div class="loading" aria-hidden="true"></div>` : nothing}
          ${rows.length === 0 && !this.table?.loading ? html`<div class="empty">Waiting for table data</div>` : html`
            <div class="canvas" style=${`height:${totalSize}px`}>
              ${virtualRows.map((virtualRow) => {
                const row = rowModel[virtualRow.index]
                const original = row.original
                const key = rowKey(original, virtualRow.index)
                const selected = key === this.selectedRowId
                return html`
                  <div
                    class=${`row ${selected ? 'selected' : ''}`}
                    role="row"
                    aria-selected=${selected ? 'true' : 'false'}
                    style=${`transform:translateY(${virtualRow.start}px)`}
                    @click=${() => {
                      this.selectedRowId = key
                      this.selectedCellKey = ''
                    }}
                  >
                    ${row.getVisibleCells().map((cell) => {
                      const column = columns.find((item) => item.key === cell.column.id) ?? { key: cell.column.id, label: cell.column.id }
                      const cellKey = `${key}:${column.key}`
                      return html`
                        <button
                          class=${`cell ${column.align === 'right' ? 'right' : ''} ${cellKey === this.selectedCellKey ? 'active' : ''}`}
                          role="cell"
                          title=${String(cell.getValue() ?? '')}
                          @click=${(event: Event) => {
                            event.stopPropagation()
                            this.selectCell(original, column, virtualRow.index)
                          }}
                        >
                          ${flexRender(cell.column.columnDef.cell, cell.getContext())}
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
          <span><strong>${rowRange}</strong></span>
          <span>${selectedText}</span>
        </div>
      </section>
    `
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
            rows: this.rows,
            columns: this.columns,
          },
        },
      }),
    )
  }

  private exportRows(): TableRow[] {
    return this.rows.map((row) => {
      const next: TableRow = {}
      for (const column of this.columns) {
        next[column.key] = formatCell(row[column.key], column)
      }
      return next
    })
  }
}

customElements.define('ld-data-table', DataTable)
