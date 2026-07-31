import { expect, test } from 'bun:test'

import {
  layoutRequirements,
  resolveWidgetLayout,
  type WidgetLayoutFeature,
} from './layout'

test('responsive layout selects the richest layout that preserves every explicit feature', () => {
  const features: WidgetLayoutFeature[] = ['comparison', 'trend']

  expect(resolveWidgetLayout('kpi', { width: 320, height: 120 }, features)).toEqual({
    kind: 'fit',
    layout: 'wide',
    minimum: { width: 320, height: 120 },
  })
  expect(resolveWidgetLayout('kpi', { width: 250, height: 130 }, features)).toEqual({
    kind: 'fit',
    layout: 'stacked',
    minimum: { width: 192, height: 124 },
  })
})

test('responsive layout rejects boundary-minus-one sizes instead of dropping features', () => {
  const features: WidgetLayoutFeature[] = ['comparison', 'trend']

  expect(resolveWidgetLayout('kpi', { width: 191, height: 124 }, features)).toEqual({
    kind: 'too-small',
    requirements: [
      { layout: 'wide', minimum: { width: 320, height: 120 } },
      { layout: 'stacked', minimum: { width: 192, height: 124 } },
    ],
  })
  expect(resolveWidgetLayout('kpi', { width: 192, height: 123 }, features).kind).toBe('too-small')
})

test('explicit KPI features deterministically increase every applicable layout minimum', () => {
  expect(layoutRequirements('kpi', [])).toEqual([
    { layout: 'wide', minimum: { width: 320, height: 80 } },
    { layout: 'stacked', minimum: { width: 192, height: 68 } },
  ])
  expect(layoutRequirements('kpi', ['subtitle', 'comparison', 'progress', 'goal', 'status', 'trend', 'note'])).toEqual([
    { layout: 'wide', minimum: { width: 320, height: 214 } },
    { layout: 'stacked', minimum: { width: 192, height: 218 } },
  ])
})

test('date-range slicers switch layout without changing their two explicit inputs', () => {
  expect(resolveWidgetLayout('slicer.date_range', { width: 268, height: 88 })).toEqual({
    kind: 'fit',
    layout: 'inline',
    minimum: { width: 268, height: 88 },
  })
  expect(resolveWidgetLayout('slicer.date_range', { width: 240, height: 136 })).toEqual({
    kind: 'fit',
    layout: 'stacked',
    minimum: { width: 172, height: 136 },
  })
  expect(resolveWidgetLayout('slicer.date_range', { width: 171, height: 136 }).kind).toBe('too-small')
})
