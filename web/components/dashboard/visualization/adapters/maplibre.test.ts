import { expect, test } from 'bun:test'

import type { VisualizationEnvelope, VisualizationGeographicLayer } from '../../../../generated/visualization'
import { coordinateGeometry, sameOriginGeometryURL, verifyGeometryDigest } from './maplibre'

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
    dataState: { kind: 'inline', datasets: [{ id: 'primary', columns: ['lat', 'lon', 'value'], rows: [[55.67, 12.56, 3], ['invalid', 12, 9]] }] },
    selection: [{ datum: { dataset: 'primary', dataRevision: 9, identity: { lat: 55.67, lon: 12.56 } }, label: 'Copenhagen' }],
  } as VisualizationEnvelope
  const geometry = coordinateGeometry(envelope, layer)
  expect(geometry.features).toHaveLength(1)
  expect(geometry.features[0]?.geometry).toEqual({ type: 'Point', coordinates: [12.56, 55.67] })
  expect(geometry.features[0]?.properties?.__ld_value).toBe(3)
  expect(geometry.features[0]?.properties?.__ld_selected).toBe(true)
})
