import { expect, test } from 'bun:test'

import type { VisualizationEnvelope, VisualizationGeographicLayer } from '../../../../generated/visualization'
import type { FeatureCollection } from 'geojson'
import { coordinateGeometry, coordinateReferenceGrid, fitMapToGeographicData, mapLayer, normalizeFeatureWeights, sameOriginGeometryURL, verifyGeometryDigest } from './maplibre'

test('MapLibre geometry assets are same-origin and content addressed', async () => {
  expect(sameOriginGeometryURL('/static/geometry/states.geojson', 'https://dash.example/workspaces/sales').href).toBe('https://dash.example/static/geometry/states.geojson')
  expect(() => sameOriginGeometryURL('https://attacker.example/states.geojson', 'https://dash.example/workspaces/sales')).toThrow(/same-origin/)
  await expect(verifyGeometryDigest(new TextEncoder().encode('geometry'), 'sha256:invalid')).rejects.toThrow(/canonical SHA-256/)
  await expect(verifyGeometryDigest(new TextEncoder().encode('geometry'), `sha256:${'0'.repeat(64)}`)).rejects.toThrow(/digest mismatch/)
})

test('MapLibre point, heat, and density layers use typed in-memory coordinates without geometry fetches', () => {
  const layer = {
    id: 'stores', kind: 'point', latitude: { dataset: 'primary', field: 'lat' }, longitude: { dataset: 'primary', field: 'lon' }, value: { dataset: 'primary', field: 'value' },
  } as VisualizationGeographicLayer
  const envelope = {
    dataRevision: 9,
    dataState: { kind: 'inline', datasets: [{ id: 'primary', columns: ['lat', 'lon', 'value'], rows: [[55.67, 12.56, 3], ['invalid', 12, 9], [91, 12, 4], [20, 181, 5]] }] },
    selection: [{ datum: { dataset: 'primary', dataRevision: 9, identity: { lat: 55.67, lon: 12.56 } }, label: 'Copenhagen' }],
  } as VisualizationEnvelope
  const geometry = coordinateGeometry(envelope, layer)
  expect(geometry.features).toHaveLength(1)
  expect(geometry.features[0]?.geometry).toEqual({ type: 'Point', coordinates: [12.56, 55.67] })
  expect(geometry.features[0]?.properties?.__ld_value).toBe(3)
  expect(geometry.features[0]?.properties?.__ld_selected).toBe(true)
})

test('MapLibre normalizes finite measure values without losing raw tooltip values', () => {
  const data = {
    type: 'FeatureCollection',
    features: [
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-70, -20] }, properties: { __ld_value: 10 } },
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-60, -10] }, properties: { __ld_value: 20 } },
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-50, 0] }, properties: { __ld_value: 30 } },
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-40, 10] }, properties: { __ld_value: null } },
    ],
  } as FeatureCollection

  const normalized = normalizeFeatureWeights(data)
  expect(normalized.features.map((feature) => feature.properties?.__ld_value)).toEqual([10, 20, 30, null])
  expect(normalized.features.map((feature) => feature.properties?.__ld_weight)).toEqual([0, 0.5, 1, 0])
})

test('MapLibre fits the combined valid feature extent with bounded padding and zoom', () => {
  const calls: unknown[][] = []
  const map = { fitBounds: (...args: unknown[]) => calls.push(args) }
  const data = {
    type: 'FeatureCollection',
    features: [
      { type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[-74, -34], [-34, -34], [-34, 5], [-74, 5], [-74, -34]]] }, properties: {} },
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-46.63, -23.55] }, properties: {} },
    ],
  } as FeatureCollection

  expect(fitMapToGeographicData(map, [data])).toBe(true)
  expect(calls).toEqual([[[[-74, -34], [-34, 5]], { padding: 24, duration: 0, maxZoom: 10 }]])
  expect(fitMapToGeographicData(map, [{ type: 'FeatureCollection', features: [] }])).toBe(false)
})

test('MapLibre coordinate maps get a bounded geographic reference grid', () => {
  const data = {
    type: 'FeatureCollection',
    features: [
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-73.9, -33.7] }, properties: {} },
      { type: 'Feature', geometry: { type: 'Point', coordinates: [-35.1, 4.2] }, properties: {} },
    ],
  } as FeatureCollection

  const grid = coordinateReferenceGrid([data])
  expect(grid.features.length).toBeGreaterThanOrEqual(8)
  expect(grid.features.every((feature) => feature.geometry.type === 'LineString')).toBe(true)
  expect(grid.features.flatMap((feature) => feature.geometry.type === 'LineString' ? feature.geometry.coordinates : [])
    .every(([longitude, latitude]) => longitude! >= -180 && longitude! <= 180 && latitude! >= -90 && latitude! <= 90)).toBe(true)
  expect(coordinateReferenceGrid([{ type: 'FeatureCollection', features: [] }]).features).toEqual([])
})

test('MapLibre heat palettes increase monotonically from transparent to dark', () => {
  for (const kind of ['heat', 'density'] as const) {
    const layer = mapLayer('observations', kind)
    expect(layer.type).toBe('heatmap')
    expect(layer.paint['heatmap-color']).toEqual([
      'interpolate', ['linear'], ['heatmap-density'],
      0, 'rgba(9,105,218,0)',
      0.15, 'rgba(84,174,255,0.28)',
      0.35, 'rgba(84,174,255,0.62)',
      0.6, '#0969da',
      0.85, '#0550ae',
      1, '#033d8b',
    ])
  }
})
