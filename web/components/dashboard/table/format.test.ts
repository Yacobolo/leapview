import { expect, test } from 'bun:test'

import { formatCell } from './format'

test('table cells use the visualization formatting contract when supplied by the IR', () => {
  expect(formatCell(1234.5, {
    key: 'revenue', label: 'Revenue', align: 'right', role: 'measure',
    visualizationFormat: { kind: 'currency', currency: 'USD' },
  })).toBe('$1,234.50')
  expect(formatCell(null, { key: 'revenue', label: 'Revenue', visualizationFormat: { kind: 'number' } })).toBe('—')
})
