import { expect, test } from 'bun:test'
import {
  filtersFromURLParams,
  interactionSelectionLabel,
  type FiltersSignal,
} from './filter-url'

test('filter URL normalization preserves typed selection mappings and identity metadata', () => {
  const filters: FiltersSignal = {
    controls: {},
    selections: [{
      entries: [{
        mappings: [
          { field: 'ratings.rating_bucket', fact: 'ratings', value: 0, label: 'Zero' },
          { field: 'activity_date', grain: 'month', value: false, label: 'false' },
          { field: 'release_decade', value: null, label: 'null' },
        ],
      }],
    }],
  }

  const normalized = filtersFromURLParams([], filters, {})
  expect(normalized.selections).toEqual(filters.selections)
  expect(normalized.selections[0]?.entries?.[0]?.mappings).toEqual([
    { field: 'ratings.rating_bucket', fact: 'ratings', value: 0, label: 'Zero' },
    { field: 'activity_date', grain: 'month', value: false, label: 'false' },
    { field: 'release_decade', value: null, label: 'null' },
  ])
})

test('interactionSelectionLabel renders typed values without dropping false, zero, or null', () => {
  expect(interactionSelectionLabel({
    entries: [{
      mappings: [
        { field: 'score', fact: 'ratings', value: 0 },
        { field: 'enabled', fact: 'ratings', value: false },
        { field: 'segment', fact: 'ratings', value: null },
      ],
    }],
  })).toBe('0, false, null')
})
