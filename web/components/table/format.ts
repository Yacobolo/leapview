import type { TableColumn, TableRow } from './types'

export function formatCell(value: unknown, column: TableColumn): string {
  if (value === null || value === undefined || value === '') return '-'
  if ((column.key === 'revenue' || column.measure === 'revenue') && Number.isFinite(Number(value))) {
    return `R$ ${Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })}`
  }
  if (column.key === 'review_score' && Number.isFinite(Number(value))) {
    return Number(value).toFixed(2)
  }
  if (column.key === 'delivery_days' && Number.isFinite(Number(value))) {
    return `${Number(value)}d`
  }
  if (Number.isFinite(Number(value)) && column.align === 'right') {
    return Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })
  }
  return String(value)
}

export function defaultDirection(column: TableColumn): 'asc' | 'desc' {
  return ['revenue', 'review_score', 'delivery_days', 'purchase_date'].includes(column.key) || column.role === 'measure' ? 'desc' : 'asc'
}

export function rowKey(row: TableRow, fallback: number): string {
  const id = row.order_id
  if (typeof id === 'string' && id) return id
  const rowID = row.__rowKey
  if (typeof rowID === 'string' && rowID) return rowID
  return String(fallback)
}
