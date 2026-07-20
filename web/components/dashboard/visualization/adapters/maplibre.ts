import type { VisualizationEnvelope, VisualizationGeographicLayer } from '../../../../generated/visualization'
import { Map as MapLibre, type Map as MapLibreMap } from 'maplibre-gl'
import type { Feature, FeatureCollection, Geometry, Position } from 'geojson'
import type { RendererAdapter, RendererHandle } from '../host-controller'

export const adapter: RendererAdapter = {
  async mount(container, envelope) {
    const frame = document.createElement('div'); frame.style.cssText = 'position:relative;width:100%;height:100%;overflow:hidden;background:var(--ld-bg-subtle,#f6f8fa)'
    const surface = document.createElement('div'); surface.style.cssText = 'position:absolute;inset:0'
    const attribution = document.createElement('div'); attribution.dataset.mapAttribution = ''; attribution.setAttribute('role', 'note'); attribution.setAttribute('aria-label', 'Map attribution')
    attribution.style.cssText = 'position:absolute;right:6px;bottom:6px;z-index:1;max-width:calc(100% - 12px);padding:2px 5px;border-radius:4px;background:color-mix(in srgb,var(--ld-bg-panel,#fff) 88%,transparent);color:var(--ld-fg-muted,#57606a);font:10px/1.3 var(--ld-font-family-ui,system-ui);pointer-events:none;text-align:right'
    frame.append(surface, attribution); container.replaceChildren(frame)
    const interactive = envelope.spec.kind === 'geographic' && envelope.spec.presentation.roam
    const backgroundColor = getComputedStyle(frame).backgroundColor || '#f6f8fa'
    const map = new MapLibre({
      container: surface,
      style: { version: 8, sources: {}, layers: [{ id: '__ld-background', type: 'background', paint: { 'background-color': backgroundColor } }] },
      attributionControl: false,
      canvasContextAttributes: { preserveDrawingBuffer: true },
      interactive,
    })
    await new Promise<void>((resolve, reject) => { map.once('load', () => resolve()); map.once('error', (event) => reject(event.error)) })
    const handle = new MapLibreHandle(container, map, attribution)
    await handle.update(envelope)
    return handle
  },
}

class MapLibreHandle implements RendererHandle {
  private sourceIDs: string[] = []
  constructor(private readonly container: HTMLElement, private readonly map: MapLibreMap, private readonly attribution: HTMLElement) {}
  async update(envelope: VisualizationEnvelope): Promise<void> {
    if (envelope.spec.kind !== 'geographic') throw new Error(`MapLibre cannot render ${envelope.spec.kind}`)
    for (const id of this.sourceIDs.reverse()) {
      if (this.map.getLayer(id)) this.map.removeLayer(id)
      if (this.map.getSource(id)) this.map.removeSource(id)
    }
    this.sourceIDs = []
    const collections: FeatureCollection[] = []
    const coordinateCollections: FeatureCollection[] = []
    const attributions = new Set<string>()
    for (const layer of envelope.spec.layers) {
      const collection = await this.addLayer(envelope, layer)
      collections.push(collection)
      if (layer.kind !== 'choropleth') coordinateCollections.push(collection)
      if (layer.geometry?.attribution) attributions.add(layer.geometry.attribution)
    }
    this.addCoordinateReferenceGrid(coordinateCollections)
    this.attribution.textContent = [...attributions].join(' · ')
    this.attribution.hidden = attributions.size === 0
    fitMapToGeographicData(this.map, collections)
    await waitForMapIdle(this.map)
  }
  resize(): void { this.map.resize() }
  async snapshot(): Promise<Blob> {
    const canvas = this.map.getCanvas()
    return new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error('MapLibre snapshot failed')), 'image/png'))
  }
  dispose(): void { this.map.remove(); this.container.replaceChildren() }

  private addCoordinateReferenceGrid(collections: FeatureCollection[]): void {
    const data = coordinateReferenceGrid(collections)
    if (data.features.length === 0) return
    let id = '__ld-coordinate-reference'
    while (this.map.getSource(id) || this.map.getLayer(id)) id += '-'
    this.map.addSource(id, { type: 'geojson', data })
    this.map.addLayer({
      id,
      source: id,
      type: 'line',
      paint: { 'line-color': '#8c959f', 'line-opacity': 0.22, 'line-width': 1, 'line-dasharray': [2, 3] },
    }, this.sourceIDs[0])
    this.sourceIDs.push(id)
  }

  private async addLayer(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer): Promise<FeatureCollection> {
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
    data = normalizeFeatureWeights(data)
    const id = `ld-${layer.id}`
    this.map.addSource(id, { type: 'geojson', data })
    this.map.addLayer(mapLayer(id, layer.kind))
    this.sourceIDs.push(id)
    return data
  }
}

function waitForMapIdle(map: MapLibreMap): Promise<void> {
  return new Promise((resolve) => {
    map.once('idle', () => resolve())
    map.triggerRepaint()
  })
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
    if (typeof latitude !== 'number' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90 || typeof longitude !== 'number' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180) continue
    features.push({ type: 'Feature', id: index, geometry: { type: 'Point', coordinates: [longitude, latitude] }, properties: {
      __ld_value: valueIndex >= 0 ? row[valueIndex] : 1,
      __ld_selected: rowIsSelected(envelope, dataset.id, dataset.columns, row),
    } })
  }
  return { type: 'FeatureCollection', features }
}

export function mapLayer(id: string, kind: VisualizationGeographicLayer['kind']): any {
  if (kind === 'choropleth') return { id, source: id, type: 'fill', paint: { 'fill-color': ['case', ['==', ['get', '__ld_value'], null], '#d8dee4', ['interpolate', ['linear'], ['get', '__ld_weight'], 0, '#ddf4ff', 0.5, '#54aeff', 1, '#0550ae']], 'fill-opacity': ['case', ['get', '__ld_selected'], 1, 0.82], 'fill-outline-color': '#ffffff' } }
  if (kind === 'point') return { id, source: id, type: 'circle', paint: { 'circle-radius': ['case', ['get', '__ld_selected'], 12, ['interpolate', ['linear'], ['get', '__ld_weight'], 0, 5, 1, 10]], 'circle-color': '#0969da', 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 1.5, 'circle-opacity': ['case', ['get', '__ld_selected'], 1, 0.78] } }
  return { id, source: id, type: 'heatmap', paint: {
    'heatmap-weight': ['*', ['get', '__ld_weight'], ['case', ['get', '__ld_selected'], 1, 0.75]],
    'heatmap-intensity': kind === 'density' ? 1.35 : 1,
    'heatmap-radius': kind === 'density' ? 24 : 32,
    'heatmap-opacity': 0.86,
    'heatmap-color': ['interpolate', ['linear'], ['heatmap-density'], 0, 'rgba(9,105,218,0)', 0.15, 'rgba(84,174,255,0.28)', 0.35, 'rgba(84,174,255,0.62)', 0.6, '#0969da', 0.85, '#0550ae', 1, '#033d8b'],
  } }
}

export function normalizeFeatureWeights(data: FeatureCollection): FeatureCollection {
  const values = data.features.map((feature) => feature.properties?.__ld_value).filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  const minimum = values.length > 0 ? Math.min(...values) : 0
  const maximum = values.length > 0 ? Math.max(...values) : 0
  const span = maximum - minimum
  return {
    ...data,
    features: data.features.map((feature) => {
      const value = feature.properties?.__ld_value
      const weight = typeof value !== 'number' || !Number.isFinite(value) ? 0 : span === 0 ? (value === 0 ? 0 : 1) : (value - minimum) / span
      return { ...feature, properties: { ...feature.properties, __ld_weight: weight } }
    }),
  }
}

type GeographicViewport = { fitBounds(bounds: [[number, number], [number, number]], options: { padding: number; duration: number; maxZoom: number }): unknown }

export function fitMapToGeographicData(map: GeographicViewport, collections: FeatureCollection[]): boolean {
  const extent = geographicExtent(collections)
  if (!extent) return false
  let [[west, south], [east, north]] = extent
  if (west === east) { west -= 0.01; east += 0.01 }
  if (south === north) { south -= 0.01; north += 0.01 }
  map.fitBounds([[west, south], [east, north]], { padding: 24, duration: 0, maxZoom: 10 })
  return true
}

export function coordinateReferenceGrid(collections: FeatureCollection[]): FeatureCollection {
  const extent = geographicExtent(collections)
  if (!extent) return { type: 'FeatureCollection', features: [] }
  const [[west, south], [east, north]] = extent
  const longitudeStep = referenceGridStep(east - west)
  const latitudeStep = referenceGridStep(north - south)
  const features: Feature<Geometry>[] = []
  for (let longitude = Math.ceil(west / longitudeStep) * longitudeStep; longitude <= east; longitude += longitudeStep) {
    features.push({ type: 'Feature', geometry: { type: 'LineString', coordinates: [[longitude, south], [longitude, north]] }, properties: {} })
  }
  for (let latitude = Math.ceil(south / latitudeStep) * latitudeStep; latitude <= north; latitude += latitudeStep) {
    features.push({ type: 'Feature', geometry: { type: 'LineString', coordinates: [[west, latitude], [east, latitude]] }, properties: {} })
  }
  return { type: 'FeatureCollection', features }
}

function referenceGridStep(span: number): number {
  const target = Math.max(span / 5, 0.0001)
  const magnitude = 10 ** Math.floor(Math.log10(target))
  const normalized = target / magnitude
  const interval = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10
  return interval * magnitude
}

function geographicExtent(collections: FeatureCollection[]): [[number, number], [number, number]] | undefined {
  let west = Infinity, south = Infinity, east = -Infinity, north = -Infinity
  const include = (position: Position) => {
    const [longitude, latitude] = position
    if (typeof longitude !== 'number' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180 || typeof latitude !== 'number' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90) return
    west = Math.min(west, longitude); east = Math.max(east, longitude); south = Math.min(south, latitude); north = Math.max(north, latitude)
  }
  const coordinates = (value: unknown): void => {
    if (!Array.isArray(value)) return
    if (value.length >= 2 && typeof value[0] === 'number' && typeof value[1] === 'number') { include(value as Position); return }
    for (const child of value) coordinates(child)
  }
  const geometry = (value: Geometry | null): void => {
    if (!value) return
    if (value.type === 'GeometryCollection') { for (const child of value.geometries) geometry(child); return }
    coordinates(value.coordinates)
  }
  for (const collection of collections) for (const feature of collection.features) geometry(feature.geometry)
  return [west, south, east, north].every(Number.isFinite) ? [[west, south], [east, north]] : undefined
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
