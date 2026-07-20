import type { VisualizationEnvelope, VisualizationGeographicLayer } from '../../../../generated/visualization'
import type { GeoJSONSource, Map as MapLibreMap } from 'maplibre-gl'
import type { Feature, FeatureCollection, Geometry } from 'geojson'
import type { RendererAdapter, RendererHandle } from '../host-controller'

export const adapter: RendererAdapter = {
  async mount(container, envelope) {
    const maplibre = await import('maplibre-gl')
    const surface = document.createElement('div'); surface.style.cssText = 'width:100%;height:100%'; container.replaceChildren(surface)
    const map = new maplibre.Map({ container: surface, style: { version: 8, sources: {}, layers: [] }, attributionControl: false })
    await new Promise<void>((resolve, reject) => { map.once('load', () => resolve()); map.once('error', (event) => reject(event.error)) })
    const handle = new MapLibreHandle(container, map)
    await handle.update(envelope)
    return handle
  },
}

class MapLibreHandle implements RendererHandle {
  private sourceIDs: string[] = []
  constructor(private readonly container: HTMLElement, private readonly map: MapLibreMap) {}
  async update(envelope: VisualizationEnvelope): Promise<void> {
    if (envelope.spec.kind !== 'geographic') throw new Error(`MapLibre cannot render ${envelope.spec.kind}`)
    for (const id of this.sourceIDs.reverse()) {
      if (this.map.getLayer(id)) this.map.removeLayer(id)
      if (this.map.getSource(id)) this.map.removeSource(id)
    }
    this.sourceIDs = []
    for (const layer of envelope.spec.layers) await this.addLayer(envelope, layer)
  }
  resize(): void { this.map.resize() }
  async snapshot(): Promise<Blob> {
    const canvas = this.map.getCanvas()
    return new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error('MapLibre snapshot failed')), 'image/png'))
  }
  dispose(): void { this.map.remove(); this.container.replaceChildren() }

  private async addLayer(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer): Promise<void> {
    let data: FeatureCollection
    if (layer.kind === 'choropleth') {
      if (!layer.geometry || !layer.join) throw new Error(`choropleth layer ${JSON.stringify(layer.id)} requires geometry and join`)
      const url = sameOriginGeometryURL(layer.geometry.url, location.href)
      const response = await fetch(url, { credentials: 'same-origin', redirect: 'error' })
      if (!response.ok) throw new Error(`geometry asset ${JSON.stringify(layer.geometry.id)} returned ${response.status}`)
      const bytes = new Uint8Array(await response.arrayBuffer())
      await verifyGeometryDigest(bytes, layer.geometry.digest)
      const geometry = JSON.parse(new TextDecoder().decode(bytes)) as FeatureCollection
      data = joinGeometry(envelope, layer, geometry)
    } else {
      data = coordinateGeometry(envelope, layer)
    }
    const id = `ld-${layer.id}`
    this.map.addSource(id, { type: 'geojson', data })
    this.map.addLayer(mapLayer(id, layer.kind))
    this.sourceIDs.push(id)
    const source = this.map.getSource(id) as GeoJSONSource
    source.setData(data)
  }
}

function joinGeometry(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer, geometry: FeatureCollection): FeatureCollection {
  if (envelope.dataState.kind !== 'inline' || !layer.join) return geometry
  const join = layer.join
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === join.dataset)
  if (!dataset) return geometry
  const joinIndex = dataset.columns.indexOf(join.field)
  const valueIndex = layer.value ? dataset.columns.indexOf(layer.value.field) : -1
  const values = new Map(dataset.rows.map((row) => [String(row[joinIndex]), {
    value: valueIndex >= 0 ? row[valueIndex] : 1,
    selected: rowIsSelected(envelope, dataset.id, dataset.columns, row),
  }]))
  const features: Feature<Geometry>[] = geometry.features.map((feature) => {
    const matched = values.get(String(feature.id ?? feature.properties?.id))
    return { ...feature, properties: { ...feature.properties, __ld_value: matched?.value ?? null, __ld_selected: matched?.selected ?? false } }
  })
  return { ...geometry, features }
}

export function coordinateGeometry(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer): FeatureCollection {
  if (envelope.dataState.kind !== 'inline' || !layer.latitude || !layer.longitude) return { type: 'FeatureCollection', features: [] }
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === layer.latitude?.dataset && candidate.id === layer.longitude?.dataset)
  if (!dataset) return { type: 'FeatureCollection', features: [] }
  const latitudeIndex = dataset.columns.indexOf(layer.latitude.field)
  const longitudeIndex = dataset.columns.indexOf(layer.longitude.field)
  const valueIndex = layer.value ? dataset.columns.indexOf(layer.value.field) : -1
  const features: Feature<Geometry>[] = []
  for (let index = 0; index < dataset.rows.length; index++) {
    const row = dataset.rows[index]!
    const latitude = row[latitudeIndex], longitude = row[longitudeIndex]
    if (typeof latitude !== 'number' || !Number.isFinite(latitude) || typeof longitude !== 'number' || !Number.isFinite(longitude)) continue
    features.push({ type: 'Feature', id: index, geometry: { type: 'Point', coordinates: [longitude, latitude] }, properties: {
      __ld_value: valueIndex >= 0 ? row[valueIndex] : 1,
      __ld_selected: rowIsSelected(envelope, dataset.id, dataset.columns, row),
    } })
  }
  return { type: 'FeatureCollection', features }
}

function mapLayer(id: string, kind: VisualizationGeographicLayer['kind']): any {
  if (kind === 'choropleth') return { id, source: id, type: 'fill', paint: { 'fill-color': ['interpolate', ['linear'], ['coalesce', ['get', '__ld_value'], 0], 0, '#eff3ff', 1, '#08519c'], 'fill-opacity': ['case', ['get', '__ld_selected'], 0.95, 0.55], 'fill-outline-color': '#ffffff' } }
  if (kind === 'point') return { id, source: id, type: 'circle', paint: { 'circle-radius': ['case', ['get', '__ld_selected'], 8, 5], 'circle-color': '#0969da', 'circle-opacity': ['case', ['get', '__ld_selected'], 1, 0.55] } }
  return { id, source: id, type: 'heatmap', paint: { 'heatmap-weight': ['*', ['coalesce', ['get', '__ld_value'], 0], ['case', ['get', '__ld_selected'], 1.5, 0.7]], 'heatmap-intensity': kind === 'density' ? 1.5 : 1 } }
}

function rowIsSelected(envelope: VisualizationEnvelope, datasetID: string, columns: string[], row: unknown[]): boolean {
  if (envelope.selection.length === 0) return false
  return envelope.selection.some(({ datum }) => {
    if (datum.dataset !== datasetID || datum.dataRevision !== envelope.dataRevision) return false
    return Object.entries(datum.identity).every(([field, value]) => {
      const index = columns.indexOf(field)
      return index >= 0 && Object.is(row[index], value)
    })
  })
}

export function sameOriginGeometryURL(value: string, base: string): URL {
  const url = new URL(value, base)
  if (url.origin !== new URL(base).origin) throw new Error('geometry asset must be same-origin')
  return url
}

export async function verifyGeometryDigest(bytes: Uint8Array, declared: string): Promise<void> {
  if (!/^sha256:[0-9a-f]{64}$/.test(declared)) throw new Error('geometry asset digest must be canonical SHA-256')
  const input = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', input))
  const actual = `sha256:${Array.from(digest, (value) => value.toString(16).padStart(2, '0')).join('')}`
  if (actual !== declared) throw new Error(`geometry asset digest mismatch: got ${actual}`)
}
