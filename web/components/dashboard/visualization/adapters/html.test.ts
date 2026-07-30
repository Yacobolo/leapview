import { expect, test } from 'bun:test'

import type { VisualizationEnvelope } from '../../../../generated/visualization'
import { defaultRendererContext } from '../host-controller'
import { accessibleLabel, kpiConditionalPresentation, kpiText } from './html'

test('HTML KPI accessible labels normalize sentence boundaries', () => {
  expect(accessibleLabel(['Revenue', 'Revenue against target.', 'Current $10.00. Target $12.00.', undefined]))
    .toBe('Revenue. Revenue against target. Current $10.00. Target $12.00.')
})

test('HTML KPI values use the field formatting contract', () => {
  const envelope = {
    schemaVersion: 8, visualID: 'revenue', rendererID: 'html', specRevision: 'sha256:test', dataRevision: 1,
    spec: {
      kind: 'kpi', title: 'Revenue', datasets: [{ id: 'primary', fields: [{ id: 'value', role: 'measure', dataType: 'decimal', nullable: false, label: 'Revenue', format: { kind: 'currency', currency: 'BRL' } }] }],
      dataBudget: { maxRows: 1, requiredCompleteness: 'complete' }, accessibility: { title: 'Revenue', description: 'Revenue' }, interactions: [],
      value: { dataset: 'primary', field: 'value' },
      presentation: { mode: 'compact', delta: 'absolute', favorableDirection: 'neutral', missingComparison: 'show_unavailable', ranges: [], tone: 'success' },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:test', dataRevision: 1, generation: 1, datasets: [{ id: 'primary', specRevision: 'sha256:test', dataRevision: 1, generation: 1, columns: ['value'], rows: [[1234.5]], completeness: 'complete' }] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  expect(kpiText(envelope)).toBe('R$1,234.50')
  expect(kpiText(envelope, { ...defaultRendererContext, locale: 'pt-BR' })).toBe('R$\u00a01.234,50')
})

test('HTML KPI formatting resolves semantic backgrounds, readable text, and redundant status cues', () => {
  const envelope = {
    schemaVersion: 8, visualID: 'health', rendererID: 'html', specRevision: 'sha256:health', dataRevision: 1,
    spec: {
      kind: 'kpi', title: 'Health', datasets: [{ id: 'primary', fields: [{ id: 'value', role: 'measure', dataType: 'decimal', nullable: false, label: 'Health' }] }],
      dataBudget: { maxRows: 1, requiredCompleteness: 'complete' }, accessibility: { title: 'Health', description: 'Health' }, interactions: [],
      conditionalFormatting: [
        {
          id: 'background', target: 'visual_background', field: { dataset: 'primary', field: 'value' },
          rule: { kind: 'rules', rules: [{ operator: 'less_than', value: 50, style: { color: 'danger', icon: 'warning' } }], nullStyle: { icon: 'warning' }, defaultStyle: { color: 'success', icon: 'arrow_up' } },
        },
        {
          id: 'value', target: 'kpi_value', field: { dataset: 'primary', field: 'value' },
          rule: { kind: 'rules', rules: [{ operator: 'less_than', value: 50, style: { color: 'warning', icon: 'arrow_down' } }], nullStyle: { icon: 'warning' }, defaultStyle: { color: 'ink', icon: 'arrow_up' } },
        },
      ],
      value: { dataset: 'primary', field: 'value' },
      presentation: { mode: 'compact', delta: 'absolute', favorableDirection: 'neutral', missingComparison: 'show_unavailable', ranges: [], tone: 'danger' },
    },
    dataState: { kind: 'inline', specRevision: 'sha256:health', dataRevision: 1, generation: 1, datasets: [{ id: 'primary', specRevision: 'sha256:health', dataRevision: 1, generation: 1, columns: ['value'], rows: [[35]], completeness: 'complete' }] },
    selection: [], status: { kind: 'ready' }, diagnostics: [],
  } as VisualizationEnvelope

  expect(kpiConditionalPresentation(envelope, defaultRendererContext)).toEqual({
    background: defaultRendererContext.colors.danger,
    foreground: defaultRendererContext.colors.surface,
    valueColor: defaultRendererContext.colors.surface,
    icon: '↓',
    iconLabel: 'decreasing',
  })
})
