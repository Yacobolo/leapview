import { expect, test } from 'bun:test'

import {
  layoutRequirements,
  resolveWidgetLayout,
  type WidgetLayoutFeature,
} from './layout'

test('responsive layout selects the richest layout that preserves every explicit feature', () => {
  const features: WidgetLayoutFeature[] = ['comparison', 'trend']

  expect(resolveWidgetLayout('kpi', { width: 320, height: 148 }, features)).toEqual({
    kind: 'fit',
    layout: 'wide',
    minimum: { width: 320, height: 148 },
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
      { layout: 'wide', minimum: { width: 320, height: 148 } },
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
    { layout: 'wide', minimum: { width: 320, height: 242 } },
    { layout: 'stacked', minimum: { width: 192, height: 218 } },
  ])
})

test('date-range slicers switch layout without changing their two explicit inputs', () => {
  expect(resolveWidgetLayout('slicer.date_range', { width: 268, height: 78 })).toEqual({
    kind: 'fit',
    layout: 'inline',
    minimum: { width: 268, height: 78 },
  })
  expect(resolveWidgetLayout('slicer.date_range', { width: 240, height: 138 })).toEqual({
    kind: 'fit',
    layout: 'stacked',
    minimum: { width: 172, height: 138 },
  })
  expect(resolveWidgetLayout('slicer.date_range', { width: 171, height: 138 }).kind).toBe('too-small')
})

test('explicit slicer summaries add one deterministic line to every layout', () => {
  expect(layoutRequirements('slicer.date_range', ['summary'])).toEqual([
    { layout: 'inline', minimum: { width: 268, height: 96 } },
    { layout: 'stacked', minimum: { width: 172, height: 156 } },
  ])
})

test('subpixel browser rounding does not demote an exact-minimum layout', () => {
  expect(resolveWidgetLayout('slicer.date_range', { width: 267.997, height: 77.997 })).toEqual({
    kind: 'fit',
    layout: 'inline',
    minimum: { width: 268, height: 78 },
  })
  expect(resolveWidgetLayout('slicer.date_range', { width: 267.4, height: 78 }).kind).toBe('too-small')
})
