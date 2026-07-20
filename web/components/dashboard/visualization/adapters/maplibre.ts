import type { VisualizationEnvelope, VisualizationGeographicLayer, VisualizationGeometryAsset, VisualizationMapCamera, VisualizationMapStyleAsset, VisualizationSpatialBounds } from '../../../../generated/visualization'
import { addProtocol, Map as MapLibre, NavigationControl, type GeoJSONSource, type Map as MapLibreMap, type MapMouseEvent, type MapOptions, type StyleSpecification } from 'maplibre-gl'
import { Protocol } from 'pmtiles'
import type { Feature, FeatureCollection, Geometry, Position } from 'geojson'
import type { OptimisticInteractionCommand } from '../../interaction-selection'
import { Change, type RendererAdapter, type RendererHandle } from '../host-controller'
import { clearInteractionCommand, interactionCommandForRowIndex } from '../interaction-command'
import { formatValue } from '../format'
import { MapSelectionControl } from './map-selection-control'

export type MapSpatialWindowRequest = {
  visualID: string; specRevision: string; dataRevision: number; requestSeq: number; resetVersion: number
  bounds: VisualizationSpatialBounds; zoom: number; width: number; height: number; windowID: string
}

export type MapObservationStage = 'basemap_load' | 'layer_shape' | 'webgl_context_loss' | 'webgl_context_restored'

export const adapter: RendererAdapter = {
  async mount(container, envelope) {
    const frame = document.createElement('div'); frame.style.cssText = 'position:relative;width:100%;height:100%;overflow:hidden;background:var(--ld-chart-surface,var(--ld-bg-panel,#fff))'
    const surface = document.createElement('div'); surface.style.cssText = 'position:absolute;inset:0'
    const attribution = document.createElement('div'); attribution.dataset.mapAttribution = ''; attribution.setAttribute('role', 'note'); attribution.setAttribute('aria-label', 'Map attribution')
    attribution.style.cssText = 'position:absolute;right:6px;bottom:6px;z-index:1;max-width:calc(100% - 12px);padding:2px 5px;border-radius:4px;background:color-mix(in srgb,var(--ld-bg-panel,#fff) 88%,transparent);color:var(--ld-fg-muted,#57606a);font:10px/1.3 var(--ld-font-family-ui,system-ui);pointer-events:none;text-align:right'
    frame.append(surface, attribution); container.replaceChildren(frame)
    const pointerOptions = mapPointerOptions(envelope)
    const backgroundColor = getComputedStyle(frame).backgroundColor || '#f6f8fa'
    const basemap = envelope.spec.kind === 'geographic' ? envelope.spec.presentation.basemap : undefined
    const basemapStarted = mapNow()
    const style = basemap ? await loadMapStyleAsset(basemap, location.href) : blankMapStyle(backgroundColor)
    emitMapObservation(frame, 'basemap_load', mapNow() - basemapStarted, envelope, { assetID: basemap?.id ?? 'blank' })
    registerPMTilesProtocol()
    const map = new MapLibre({
      container: surface,
      style,
      attributionControl: false,
      canvasContextAttributes: { preserveDrawingBuffer: true },
      ...pointerOptions,
    })
    await new Promise<void>((resolve) => { map.once('load', () => resolve()) })
    const handle = new MapLibreHandle(container, frame, map, attribution)
    try {
      await handle.update(envelope)
      return handle
    } catch (error) {
      handle.dispose()
      throw error
    }
  },
}

export function mapPointerOptions(envelope: VisualizationEnvelope): Pick<MapOptions, 'interactive' | 'scrollZoom' | 'boxZoom' | 'dragRotate' | 'dragPan' | 'keyboard' | 'doubleClickZoom' | 'touchZoomRotate' | 'touchPitch'> {
  const geographic = envelope.spec.kind === 'geographic'
  const roam = envelope.spec.kind === 'geographic' ? envelope.spec.presentation.roam : false
  const selectable = geographic && envelope.spec.interactions.some((candidate) => candidate.kind === 'select')
  return {
    interactive: roam || selectable,
    scrollZoom: roam,
    boxZoom: roam,
    dragRotate: roam,
    dragPan: roam,
    keyboard: roam,
    doubleClickZoom: roam,
    touchZoomRotate: roam,
    touchPitch: roam,
  }
}

class MapLibreHandle implements RendererHandle {
  private sourceIDs: string[] = []
  private layerIDs: string[] = []
  private dynamicLayers: Array<{ spec: VisualizationGeographicLayer; sourceID: string; geometry?: FeatureCollection }> = []
  private selectableLayerIDs: string[] = []
  private tooltipLayerIDs: string[] = []
  private clusterLayerIDs: string[] = []
  private clusterSources = new Map<string, string>()
  private envelope?: VisualizationEnvelope
  private selectionControl?: MapSelectionControl
  private navigationControl?: NavigationControl
  private resetButton?: HTMLButtonElement
  private readonly tooltip: HTMLDivElement
  private readonly legend: HTMLDivElement
  private readonly accessibleTable: HTMLDetailsElement
  private homeCamera?: { center: [number, number]; zoom: number; bearing: number; pitch: number }
  private updateQueue: Promise<void> = Promise.resolve()
  private spatialRequestSeq = 0
  private spatialRequestTimer?: number
  private disposed = false
  private readonly disposeWebGLRecovery: () => void
  private readonly handleThemeApplied = () => this.applyTheme()
  constructor(private readonly container: HTMLElement, private readonly frame: HTMLElement, private readonly map: MapLibreMap, private readonly attribution: HTMLElement) {
    this.tooltip = document.createElement('div')
    this.tooltip.setAttribute('role', 'tooltip')
    this.tooltip.hidden = true
    this.tooltip.style.cssText = 'position:absolute;z-index:4;max-width:280px;padding:8px 10px;border:1px solid var(--ld-line-default,#d0d7de);border-radius:6px;background:color-mix(in srgb,var(--ld-bg-panel,#fff) 96%,transparent);box-shadow:var(--ld-shadow-floating,0 8px 24px rgba(140,149,159,.2));color:var(--ld-fg-default,#1f2328);font:12px/1.45 var(--ld-font-family-ui,system-ui);pointer-events:none'
    this.legend = document.createElement('div')
    this.legend.setAttribute('role', 'note')
    this.legend.dataset.mapLegend = ''
    this.legend.hidden = true
    this.legend.style.cssText = 'position:absolute;z-index:3;right:10px;bottom:28px;min-width:132px;max-width:220px;padding:8px;border:1px solid var(--ld-line-default,#d0d7de);border-radius:6px;background:color-mix(in srgb,var(--ld-bg-panel,#fff) 94%,transparent);color:var(--ld-fg-default,#1f2328);font:11px/1.35 var(--ld-font-family-ui,system-ui)'
    this.accessibleTable = document.createElement('details')
    this.accessibleTable.dataset.mapDataTable = ''
    this.accessibleTable.style.cssText = 'position:absolute;z-index:3;left:10px;bottom:28px;max-width:min(520px,calc(100% - 20px));max-height:55%;overflow:auto;border:1px solid var(--ld-line-default,#d0d7de);border-radius:6px;background:color-mix(in srgb,var(--ld-bg-panel,#fff) 96%,transparent);color:var(--ld-fg-default,#1f2328);font:11px/1.35 var(--ld-font-family-ui,system-ui);box-shadow:0 1px 3px rgba(31,35,40,.12)'
    this.frame.append(this.tooltip, this.legend, this.accessibleTable)
    document.addEventListener('libredash-theme-applied', this.handleThemeApplied)
    this.map.on('click', this.handleClick)
    this.map.on('mousemove', this.handlePointerMove)
    this.map.on('mouseout', this.handlePointerLeave)
    this.map.on('moveend', this.handleMoveEnd)
    this.disposeWebGLRecovery = installWebGLRecovery(this.map.getCanvas(), this.map, (stage) => {
      if (this.envelope) emitMapObservation(this.frame, stage, 0, this.envelope)
    })
  }
  update(envelope: VisualizationEnvelope, change: Change = Change.All): Promise<void> {
    if (this.disposed) return Promise.resolve()
    const pending = this.updateQueue.then(() => this.applyUpdate(envelope, change))
    this.updateQueue = pending.catch(() => {})
    return pending
  }
  private async applyUpdate(envelope: VisualizationEnvelope, change: Change): Promise<void> {
    if (this.disposed) return
    if (envelope.spec.kind !== 'geographic') throw new Error(`MapLibre cannot render ${envelope.spec.kind}`)
    this.envelope = envelope
    this.updateAccessibleFallback(envelope)
    this.map.setMinZoom(envelope.spec.presentation.camera.minimumZoom)
    this.map.setMaxZoom(envelope.spec.presentation.camera.maximumZoom)
    this.applyTheme()
    this.updateSelectionControl(envelope)
    if ((change & (Change.Spec | Change.Data)) === 0) {
      if ((change & Change.Selection) !== 0) this.updateSelectionData(envelope)
      return
    }
    if ((change & Change.Spec) === 0 && (change & Change.Data) !== 0 && this.dynamicLayers.length > 0) {
      this.updateSelectionData(envelope)
      this.updateLegend(envelope)
      return
    }
    this.removeOwnedMapData()
    this.sourceIDs = []
    this.layerIDs = []
    this.dynamicLayers = []
    this.selectableLayerIDs = []
    this.tooltipLayerIDs = []
    this.clusterLayerIDs = []
    this.clusterSources.clear()
    const collections: FeatureCollection[] = []
    const coordinateCollections: FeatureCollection[] = []
    const attributions = new Set<string>()
    if (envelope.spec.presentation.basemap) attributions.add(envelope.spec.presentation.basemap.attribution)
    for (const layer of envelope.spec.layers) {
      const shapeStarted = mapNow()
      const collection = await this.addLayer(envelope, layer)
      emitMapObservation(this.frame, 'layer_shape', mapNow() - shapeStarted, envelope, { layerID: layer.id, featureCount: collection.features.length })
      if (this.disposed) return
      collections.push(collection)
      if (layer.kind !== 'choropleth') coordinateCollections.push(collection)
      if ('geometry' in layer && layer.geometry.attribution) attributions.add(layer.geometry.attribution)
    }
    if (!envelope.spec.presentation.basemap) this.addCoordinateReferenceGrid(coordinateCollections)
    this.attribution.textContent = [...attributions].join(' · ')
    this.attribution.hidden = attributions.size === 0
    fitMapToGeographicData(this.map, collections, envelope.spec.presentation.camera)
    this.captureHomeCamera()
    this.updateMapControls(envelope)
    this.updateLegend(envelope)
    this.handleMoveEnd()
    if (this.disposed) return
    await waitForMapIdle(this.map)
  }
  resize(): void { this.map.resize() }
  async snapshot(): Promise<Blob> {
    await waitForMapIdle(this.map)
    const canvas = this.map.getCanvas()
    return new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error('MapLibre snapshot failed')), 'image/png'))
  }
  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    document.removeEventListener('libredash-theme-applied', this.handleThemeApplied)
    this.map.off('click', this.handleClick)
    this.map.off('mousemove', this.handlePointerMove)
    this.map.off('mouseout', this.handlePointerLeave)
    this.map.off('moveend', this.handleMoveEnd)
    this.disposeWebGLRecovery()
    if (this.spatialRequestTimer !== undefined) window.clearTimeout(this.spatialRequestTimer)
    this.selectionControl?.dispose()
    if (this.navigationControl) this.map.removeControl(this.navigationControl)
    this.resetButton?.remove()
    this.map.remove()
    removeRendererFrame(this.container, this.frame)
  }

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
    this.layerIDs.push(id)
  }

  private async addLayer(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer): Promise<FeatureCollection> {
    let data: FeatureCollection
    let geometry: FeatureCollection | undefined
    if (layer.kind === 'choropleth') {
      if (!layer.geometry || !layer.join) throw new Error(`choropleth layer ${JSON.stringify(layer.id)} requires geometry and join`)
      geometry = await this.loadGeometry(layer.geometry)
      if (this.disposed) return { type: 'FeatureCollection', features: [] }
      data = joinGeometry(envelope, layer, geometry)
    } else if (layer.kind === 'reference') {
      geometry = await this.loadGeometry(layer.geometry)
      data = geometry
    } else if (layer.kind === 'path') {
      data = pathGeometry(envelope, layer)
    } else {
      data = coordinateGeometry(envelope, layer)
    }
    data = applyFeatureScales(data, layer)
    const id = `ld-${layer.id}`
    const sourceOptions: any = { type: 'geojson', data }
    if (layer.kind === 'point' && layer.cluster.enabled) Object.assign(sourceOptions, { cluster: true, clusterRadius: layer.cluster.radius, clusterMaxZoom: layer.cluster.maximumZoom, clusterMinPoints: layer.cluster.minimumPoints })
    this.map.addSource(id, sourceOptions)
    const before = layer.position === 'below_labels' && envelope.spec.kind === 'geographic' && envelope.spec.presentation.basemap?.labelAnchor && this.map.getLayer(envelope.spec.presentation.basemap.labelAnchor)
      ? envelope.spec.presentation.basemap.labelAnchor : undefined
    this.map.addLayer(mapLayer(id, layer), before)
    this.sourceIDs.push(id)
    this.layerIDs.push(id)
    if (layer.kind === 'reference') {
      const lineID = `${id}-line`, pointID = `${id}-point`
      this.map.addLayer({ id: lineID, source: id, type: 'line', filter: ['==', ['geometry-type'], 'LineString'], minzoom: layer.visibility.minimumZoom, maxzoom: layer.visibility.maximumZoom, paint: {
        'line-color': layer.stroke.color, 'line-width': layer.stroke.width, 'line-opacity': layer.opacity * layer.stroke.opacity,
      } }, before)
      this.map.addLayer({ id: pointID, source: id, type: 'circle', filter: ['==', ['geometry-type'], 'Point'], minzoom: layer.visibility.minimumZoom, maxzoom: layer.visibility.maximumZoom, paint: {
        'circle-color': paletteColors(layer.color)[2], 'circle-radius': Math.max(3, layer.stroke.width * 2), 'circle-opacity': layer.opacity,
        'circle-stroke-color': layer.stroke.color, 'circle-stroke-width': layer.stroke.width, 'circle-stroke-opacity': layer.stroke.opacity,
      } }, before)
      this.layerIDs.push(lineID, pointID)
    }
    if (layer.kind === 'point' && layer.cluster.enabled) this.addClusterLayers(id, layer, before)
    if (layer.label && (layer.kind === 'point' || layer.kind === 'choropleth')) this.addDataLabelLayer(id, layer, envelope.spec.kind === 'geographic' ? envelope.spec.presentation.theme : 'auto')
    if (layer.kind === 'choropleth') {
      const outlineID = `${id}-selected-outline`
      this.map.addLayer(mapOutlineLayer(outlineID, id))
      this.layerIDs.push(outlineID)
    }
    if (layer.kind === 'point' || layer.kind === 'choropleth') this.selectableLayerIDs.push(id)
    if (layer.tooltip.length > 0 && layer.kind !== 'reference') this.tooltipLayerIDs.push(id)
    this.dynamicLayers.push({ spec: layer, sourceID: id, geometry })
    return data
  }

  private updateSelectionData(envelope: VisualizationEnvelope): void {
    updateSelectionSources(envelope, this.dynamicLayers, (sourceID) => this.map.getSource(sourceID) as GeoJSONSource | undefined)
    this.map.triggerRepaint()
  }

  private removeOwnedMapData(): void {
    for (const id of [...this.layerIDs].reverse()) if (this.map.getLayer(id)) this.map.removeLayer(id)
    for (const id of [...this.sourceIDs].reverse()) if (this.map.getSource(id)) this.map.removeSource(id)
  }

  private updateSelectionControl(envelope: VisualizationEnvelope): void {
    const selectable = envelope.spec.interactions.some((candidate) => candidate.kind === 'select')
    if (!selectable) {
      this.selectionControl?.dispose()
      this.selectionControl = undefined
      return
    }
    this.selectionControl ??= new MapSelectionControl((command) => this.dispatchInteraction(command))
    if (!this.selectionControl.element.isConnected) this.frame.append(this.selectionControl.element)
    this.selectionControl.update(envelope)
  }

  private addClusterLayers(sourceID: string, layer: Extract<VisualizationGeographicLayer, { kind: 'point' }>, before?: string): void {
    const clusterID = `${sourceID}-clusters`, countID = `${sourceID}-cluster-count`
    this.map.addLayer({ id: clusterID, source: sourceID, type: 'circle', filter: ['has', 'point_count'], minzoom: layer.visibility.minimumZoom, maxzoom: layer.visibility.maximumZoom, paint: {
      'circle-color': '#0969da', 'circle-opacity': 0.88, 'circle-stroke-color': layer.stroke.color, 'circle-stroke-width': Math.max(layer.stroke.width, 1.5),
      'circle-radius': ['step', ['get', 'point_count'], 14, 10, 18, 50, 23, 250, 29],
    } }, before)
    this.map.addLayer({ id: countID, source: sourceID, type: 'symbol', filter: ['has', 'point_count'], minzoom: layer.visibility.minimumZoom, maxzoom: layer.visibility.maximumZoom, layout: {
      'text-field': layer.cluster.showCount ? ['get', 'point_count_abbreviated'] : '', 'text-font': ['Noto Sans Medium'], 'text-size': 11,
    }, paint: { 'text-color': '#ffffff', 'text-halo-color': '#0550ae', 'text-halo-width': 0.5 } })
    this.layerIDs.push(clusterID, countID)
    this.clusterLayerIDs.push(countID, clusterID)
    this.clusterSources.set(clusterID, sourceID)
    this.clusterSources.set(countID, sourceID)
  }

  private addDataLabelLayer(sourceID: string, layer: Extract<VisualizationGeographicLayer, { kind: 'point' | 'choropleth' }>, theme: 'auto' | 'light' | 'dark'): void {
    const id = `${sourceID}-data-label`
    this.map.addLayer({ id, source: sourceID, type: 'symbol', filter: layer.kind === 'point' ? ['all', ['!', ['has', 'point_count']], ['!=', ['get', '__ld_label'], '']] : ['!=', ['get', '__ld_label'], ''], minzoom: layer.visibility.minimumZoom, maxzoom: layer.visibility.maximumZoom, layout: {
      'text-field': ['get', '__ld_label'], 'text-font': ['Noto Sans Medium'], 'text-size': 11, 'text-offset': [0, layer.kind === 'point' ? 1.25 : 0], 'text-anchor': layer.kind === 'point' ? 'top' : 'center', 'text-optional': true,
    }, paint: { 'text-color': theme === 'dark' ? '#f0f6fc' : '#1f2328', 'text-halo-color': theme === 'dark' ? '#0d1821' : '#ffffff', 'text-halo-width': 1.25 } })
    this.layerIDs.push(id)
  }

  private updateTooltip(event: MapMouseEvent, features: readonly RenderedFeatureLocator[]): void {
    if (!this.envelope) return
    const entries = mapTooltipEntries(this.envelope, features)
    if (!entries.length) { this.tooltip.hidden = true; return }
    const fragment = document.createDocumentFragment()
    for (const entry of entries) {
      const row = document.createElement('div'); row.style.cssText = 'display:grid;grid-template-columns:minmax(64px,auto) minmax(0,1fr);gap:10px'
      const label = document.createElement('span'); label.style.color = 'var(--ld-fg-muted,#57606a)'; label.textContent = entry.label
      const value = document.createElement('strong'); value.style.cssText = 'font-weight:600;text-align:right;overflow-wrap:anywhere'; value.textContent = entry.value
      row.append(label, value); fragment.append(row)
    }
    this.tooltip.replaceChildren(fragment)
    this.tooltip.hidden = false
    this.tooltip.style.left = `${Math.min(event.point.x + 12, Math.max(8, this.frame.clientWidth - 292))}px`
    this.tooltip.style.top = `${Math.min(event.point.y + 12, Math.max(8, this.frame.clientHeight - this.tooltip.offsetHeight - 8))}px`
  }

  private updateMapControls(envelope: VisualizationEnvelope): void {
    if (envelope.spec.kind !== 'geographic' || this.navigationControl || this.resetButton) return
    const controls = envelope.spec.presentation.controls
    if (controls.zoom || controls.compass) {
      this.navigationControl = new NavigationControl({ showZoom: controls.zoom, showCompass: controls.compass, visualizePitch: false })
      this.map.addControl(this.navigationControl, 'top-right')
    }
    if (controls.reset) {
      const button = document.createElement('button')
      button.type = 'button'; button.className = 'ld-map-reset'; button.textContent = 'Reset view'; button.setAttribute('aria-label', 'Reset map view')
      button.style.cssText = 'position:absolute;z-index:3;top:10px;right:50px;padding:5px 8px;border:1px solid var(--ld-line-default,#d0d7de);border-radius:4px;background:var(--ld-bg-panel,#fff);color:var(--ld-fg-default,#1f2328);font:600 11px/1.2 var(--ld-font-family-ui,system-ui);cursor:pointer;box-shadow:0 1px 2px rgba(31,35,40,.08)'
      button.addEventListener('click', () => { if (this.homeCamera) this.map.easeTo(this.homeCamera) })
      this.frame.append(button); this.resetButton = button
    }
  }

  private captureHomeCamera(): void {
    const center = this.map.getCenter()
    this.homeCamera = { center: [center.lng, center.lat], zoom: this.map.getZoom(), bearing: this.map.getBearing(), pitch: this.map.getPitch() }
  }

  private updateLegend(envelope: VisualizationEnvelope): void {
    if (envelope.spec.kind !== 'geographic' || envelope.spec.presentation.legend === 'hidden') { this.legend.hidden = true; return }
    const rows: HTMLElement[] = []
    for (const layer of envelope.spec.layers) {
      const value = 'value' in layer ? layer.value : undefined
      const category = 'category' in layer ? layer.category : undefined
      const field = value ?? category
      if (!field) continue
      const schema = envelope.spec.datasets.find((candidate) => candidate.id === field.dataset)
      const definition = schema?.fields.find((candidate) => candidate.id === field.field)
      const item = document.createElement('div'); item.style.cssText = 'display:grid;gap:4px;margin-bottom:7px'
      const title = document.createElement('strong'); title.textContent = definition?.label ?? field.field
      const colors = 'color' in layer ? paletteColors(layer.color) : paletteColors()
      const scale = document.createElement('span'); scale.style.cssText = `display:block;width:100%;height:8px;border-radius:999px;background:linear-gradient(90deg,${colors.join(',')})`
      item.append(title, scale); rows.push(item)
    }
    this.legend.replaceChildren(...rows); this.legend.hidden = rows.length === 0
    const position = envelope.spec.presentation.legend
    this.legend.style.left = position === 'left' ? '10px' : ''
    this.legend.style.right = position === 'right' ? '10px' : ''
    this.legend.style.top = position === 'top' ? '10px' : ''
    this.legend.style.bottom = position === 'bottom' ? '28px' : position === 'top' ? '' : '28px'
  }

  private updateAccessibleFallback(envelope: VisualizationEnvelope): void {
    const data = mapAccessibleData(envelope)
    const summary = document.createElement('summary')
    summary.textContent = `View map data (${data.rows.length}${data.totalRows > data.rows.length ? ` of ${data.totalRows}` : ''} rows)`
    summary.style.cssText = 'padding:6px 8px;cursor:pointer;font-weight:600;white-space:nowrap'
    const table = document.createElement('table')
    table.style.cssText = 'border-collapse:collapse;min-width:100%;background:var(--ld-bg-panel,#fff)'
    const caption = document.createElement('caption')
    caption.textContent = envelope.spec.accessibility.summary ?? envelope.spec.accessibility.description
    caption.style.cssText = 'padding:6px 8px;text-align:left;color:var(--ld-fg-muted,#57606a)'
    const header = document.createElement('tr')
    for (const column of data.columns) {
      const cell = document.createElement('th'); cell.scope = 'col'; cell.textContent = column.label
      cell.style.cssText = 'padding:5px 8px;border-top:1px solid var(--ld-line-subtle,#d8dee4);border-bottom:1px solid var(--ld-line-default,#d0d7de);text-align:left;white-space:nowrap'
      header.append(cell)
    }
    const head = document.createElement('thead'); head.append(header)
    const body = document.createElement('tbody')
    for (const row of data.rows) {
      const element = document.createElement('tr')
      for (const value of row) {
        const cell = document.createElement('td'); cell.textContent = value
        cell.style.cssText = 'padding:4px 8px;border-bottom:1px solid var(--ld-line-subtle,#d8dee4);white-space:nowrap'
        element.append(cell)
      }
      body.append(element)
    }
    table.append(caption, head, body)
    this.accessibleTable.replaceChildren(summary, table)
  }

  private dispatchInteraction(command: OptimisticInteractionCommand): void {
    this.container.dispatchEvent(new CustomEvent('ld-interaction-select', { bubbles: true, composed: true, detail: command }))
  }

  private readonly handleClick = (event: MapMouseEvent) => {
    if (!this.envelope || this.selectableLayerIDs.length === 0) return
    const clusters = this.clusterLayerIDs.length ? this.map.queryRenderedFeatures(event.point, { layers: this.clusterLayerIDs }) : []
    const cluster = clusters[0]
    const clusterID = cluster?.properties?.cluster_id
    const sourceID = cluster?.layer?.id ? this.clusterSources.get(cluster.layer.id) : undefined
    if (typeof clusterID === 'number' && sourceID) {
      const source = this.map.getSource(sourceID) as GeoJSONSource | undefined
      void source?.getClusterExpansionZoom(clusterID).then((zoom) => this.map.easeTo({ center: (cluster.geometry as any).coordinates, zoom }))
      return
    }
    const features = this.map.queryRenderedFeatures(event.point, { layers: this.selectableLayerIDs })
    const command = mapInteractionCommand(this.envelope, features, this.selectableLayerIDs)
    if (command) this.dispatchInteraction(command)
  }

  private readonly handleMoveEnd = () => {
    if (!this.envelope || this.envelope.dataState.kind !== 'spatial_windowed') return
    if (this.spatialRequestTimer !== undefined) window.clearTimeout(this.spatialRequestTimer)
    this.spatialRequestTimer = window.setTimeout(() => {
      this.spatialRequestTimer = undefined
      if (!this.envelope || this.envelope.dataState.kind !== 'spatial_windowed' || this.disposed) return
      const bounds = this.map.getBounds()
      const request = spatialWindowRequest(this.envelope, {
        west: bounds.getWest(), south: bounds.getSouth(), east: bounds.getEast(), north: bounds.getNorth(),
      }, this.map.getZoom(), this.map.getCanvas().clientWidth, this.map.getCanvas().clientHeight, ++this.spatialRequestSeq)
      if (!request) return
      this.container.dispatchEvent(new CustomEvent('ld-visual-spatial-window-change', { bubbles: true, composed: true, detail: request }))
    }, 120)
  }

  private readonly handlePointerMove = (event: MapMouseEvent) => {
    if (!this.envelope) return
    const layers = [...new Set([...this.selectableLayerIDs, ...this.tooltipLayerIDs])]
    if (layers.length === 0) return
    const features = this.map.queryRenderedFeatures(event.point, { layers })
    this.map.getCanvas().style.cursor = interactionCommandForRenderedFeatures(this.envelope, features, this.selectableLayerIDs) ? 'pointer' : ''
    this.updateTooltip(event, features)
  }

  private readonly handlePointerLeave = () => { this.map.getCanvas().style.cursor = ''; this.tooltip.hidden = true }

  private async loadGeometry(asset: VisualizationGeometryAsset): Promise<FeatureCollection> {
    return loadGeometryAsset(asset, location.href)
  }

  private applyTheme(): void {
    const labelDensity = this.envelope?.spec.kind === 'geographic' ? this.envelope.spec.presentation.labelDensity : 'normal'
    applyBasemapTheme(this.map, this.currentBasemapColors(), getComputedStyle(this.frame).backgroundColor || '#ffffff', labelDensity)
    this.map.triggerRepaint()
  }

  private currentBasemapColors(): BasemapColors {
    const theme = this.envelope?.spec.kind === 'geographic' ? this.envelope.spec.presentation.theme : 'auto'
    const resolved = getComputedStyle(document.documentElement).colorScheme.includes('dark') ? 'dark' : 'light'
    return mapThemeColors(theme, resolved)
  }
}

export function installWebGLRecovery(
  canvas: EventTarget,
  map: Pick<MapLibreMap, 'resize' | 'triggerRepaint'>,
  observe: (stage: Extract<MapObservationStage, 'webgl_context_loss' | 'webgl_context_restored'>) => void = () => {},
): () => void {
  const lost = (event: Event) => {
    event.preventDefault()
    observe('webgl_context_loss')
  }
  const restored = () => {
    map.resize()
    map.triggerRepaint()
    observe('webgl_context_restored')
  }
  canvas.addEventListener('webglcontextlost', lost)
  canvas.addEventListener('webglcontextrestored', restored)
  return () => {
    canvas.removeEventListener('webglcontextlost', lost)
    canvas.removeEventListener('webglcontextrestored', restored)
  }
}

function emitMapObservation(
  target: EventTarget,
  stage: MapObservationStage,
  durationMs: number,
  envelope: VisualizationEnvelope,
  detail: Readonly<Record<string, string | number>> = {},
): void {
  target.dispatchEvent(new CustomEvent('ld-map-observation', {
    bubbles: true,
    composed: true,
    detail: { stage, durationMs, visualID: envelope.visualID, rendererID: envelope.rendererID, ...detail },
  }))
}

function mapNow(): number { return typeof performance === 'undefined' ? Date.now() : performance.now() }

const geometryCache = new Map<string, Promise<FeatureCollection>>()
const mapStyleCache = new Map<string, Promise<StyleSpecification>>()
let pmtilesRegistered = false

function registerPMTilesProtocol(): void {
  if (pmtilesRegistered) return
  const protocol = new Protocol()
  addProtocol('pmtiles', protocol.tile)
  pmtilesRegistered = true
}

function blankMapStyle(background: string): StyleSpecification {
  return { version: 8, sources: {}, layers: [{ id: '__ld-background', type: 'background', metadata: { 'libredash:role': 'background' }, paint: { 'background-color': background } }] }
}

export async function loadMapStyleAsset(asset: VisualizationMapStyleAsset, baseURL: string): Promise<StyleSpecification> {
  const styleURL = sameOriginGeometryURL(asset.styleUrl, baseURL)
  const archiveURL = sameOriginGeometryURL(asset.archiveUrl, baseURL)
  const glyphsURL = sameOriginGeometryURL(asset.glyphsUrl, baseURL)
  const spriteURL = sameOriginGeometryURL(asset.spriteUrl, baseURL)
  const key = `${styleURL.href}\0${asset.styleDigest}\0${archiveURL.href}\0${asset.archiveDigest}`
  let pending = mapStyleCache.get(key)
  if (!pending) {
    pending = (async () => {
      const response = await fetch(styleURL, { credentials: 'same-origin', redirect: 'error' })
      if (!response.ok) throw new Error(`map style asset ${JSON.stringify(asset.id)} returned ${response.status}`)
      const bytes = new Uint8Array(await response.arrayBuffer())
      await verifyGeometryDigest(bytes, asset.styleDigest)
      const style = JSON.parse(new TextDecoder().decode(bytes)) as StyleSpecification
      if (style.version !== 8 || !style.sources || !Array.isArray(style.layers)) throw new Error(`map style asset ${JSON.stringify(asset.id)} is not a MapLibre style`)
      for (const source of Object.values(style.sources) as Array<{ url?: string }>) {
        if (source.url === 'pmtiles://__LIBREDASH_ARCHIVE__') source.url = `pmtiles://${archiveURL.href}`
      }
      // URL parsing deliberately escapes braces, while MapLibre requires both
      // glyph placeholders to remain literal after the URL has been validated.
      style.glyphs = glyphsURL.href
        .replace(/%7Bfontstack%7D/gi, '{fontstack}')
        .replace(/%7Brange%7D/gi, '{range}')
      style.sprite = spriteURL.href
      return style
    })()
    mapStyleCache.set(key, pending)
    void pending.catch(() => { if (mapStyleCache.get(key) === pending) mapStyleCache.delete(key) })
  }
  return structuredClone(await pending)
}

async function loadGeometryAsset(asset: VisualizationGeometryAsset, baseURL: string): Promise<FeatureCollection> {
  const url = sameOriginGeometryURL(asset.url, baseURL)
  const key = `${url.href}\0${asset.digest}`
  let pending = geometryCache.get(key)
  if (!pending) {
    pending = (async () => {
      const response = await fetch(url, { credentials: 'same-origin', redirect: 'error' })
      if (!response.ok) throw new Error(`geometry asset ${JSON.stringify(asset.id)} returned ${response.status}`)
      const bytes = new Uint8Array(await response.arrayBuffer())
      await verifyGeometryDigest(bytes, asset.digest)
      const value = JSON.parse(new TextDecoder().decode(bytes)) as Partial<FeatureCollection>
      if (value.type !== 'FeatureCollection' || !Array.isArray(value.features)) throw new Error(`geometry asset ${JSON.stringify(asset.id)} is not a GeoJSON FeatureCollection`)
      return value as FeatureCollection
    })()
    geometryCache.set(key, pending)
    void pending.catch(() => { if (geometryCache.get(key) === pending) geometryCache.delete(key) })
  }
  return pending
}

type BasemapColors = Readonly<{ boundary: string; land: string; background?: string; water?: string; road?: string; building?: string; label?: string }>

export function mapThemeColors(theme: 'auto' | 'light' | 'dark', resolved: 'light' | 'dark'): BasemapColors {
  const effective = theme === 'auto' ? resolved : theme
  if (effective === 'dark') return { background: '#0d1821', land: '#18232d', water: '#0d1821', boundary: '#657383', road: '#394957', building: '#263540', label: '#d6dee6' }
  return { background: '#aad3df', land: '#f4f1ea', water: '#aad3df', boundary: '#8f918d', road: '#ffffff', building: '#dedbd4', label: '#4b4d49' }
}

export function basemapLayer(id: string, colors: BasemapColors): any {
  return { id, source: id, type: 'fill', paint: { 'fill-color': colors.land, 'fill-opacity': 1 } }
}

export function basemapBoundaryLayer(id: string, source: string, boundary: string): any {
  return { id, source, type: 'line', paint: { 'line-color': boundary, 'line-opacity': 0.92, 'line-width': 1.5 } }
}

export function removeRendererFrame(container: ParentNode, frame: HTMLElement): void {
  if (frame.parentNode === container) frame.remove()
}

function resolveCSSColor(container: HTMLElement, value: string, fallback: string): string {
  const probe = document.createElement('span')
  probe.style.color = value
  probe.hidden = true
  container.append(probe)
  const color = getComputedStyle(probe).color
  probe.remove()
  return concreteCSSColor(color, fallback)
}

export function concreteCSSColor(resolved: string, fallback: string): string {
  return resolved.trim() || fallback
}

function waitForMapIdle(map: MapLibreMap): Promise<void> {
  return new Promise((resolve) => {
    map.once('idle', () => resolve())
    map.triggerRepaint()
  })
}

export function joinGeometry(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer, geometry: FeatureCollection): FeatureCollection {
  if (envelope.dataState.kind !== 'inline' || layer.kind !== 'choropleth') return geometry
  const join = layer.join
  const dataset = envelope.dataState.datasets.find((candidate) => candidate.id === join.dataset)
  if (!dataset) return geometry
  const joinIndex = dataset.columns.indexOf(join.field)
  const valueIndex = layer.value ? dataset.columns.indexOf(layer.value.field) : -1
  const categoryIndex = layer.category ? dataset.columns.indexOf(layer.category.field) : -1
  const labelIndex = layer.label ? dataset.columns.indexOf(layer.label.field) : -1
  const values = new Map(dataset.rows.map((row, rowIndex) => [String(row[joinIndex]), {
    value: valueIndex >= 0 ? row[valueIndex] : 1,
    category: categoryIndex >= 0 ? row[categoryIndex] : null,
    label: labelIndex >= 0 ? String(row[labelIndex] ?? '') : '',
    selected: rowIsSelected(envelope, dataset.id, dataset.columns, row),
    rowIndex,
  }]))
  const features: Feature<Geometry>[] = geometry.features.map((feature) => {
    const matched = values.get(String(feature.id ?? feature.properties?.id))
    return { ...feature, properties: {
      ...feature.properties,
      __ld_value: matched?.value ?? null,
      __ld_category: matched?.category ?? null,
      __ld_label: matched?.label ?? '',
      __ld_selected: matched?.selected ?? false,
      __ld_has_selection: envelope.selection.length > 0,
      ...(matched ? rowLocator(dataset.id, matched.rowIndex, layer.id) : {}),
    } }
  })
  return { ...geometry, features }
}

export function coordinateGeometry(envelope: VisualizationEnvelope, layer: VisualizationGeographicLayer): FeatureCollection {
  if (!['point', 'heat', 'density'].includes(layer.kind)) return { type: 'FeatureCollection', features: [] }
  const coordinateLayer = layer as Extract<VisualizationGeographicLayer, { kind: 'point' | 'heat' | 'density' }>
  const dataset = geographicDataset(envelope, coordinateLayer.latitude.dataset)
  if (coordinateLayer.latitude.dataset !== coordinateLayer.longitude.dataset) return { type: 'FeatureCollection', features: [] }
  if (!dataset) return { type: 'FeatureCollection', features: [] }
  const latitudeIndex = dataset.columns.indexOf(coordinateLayer.latitude.field)
  const longitudeIndex = dataset.columns.indexOf(coordinateLayer.longitude.field)
  const valueIndex = coordinateLayer.value ? dataset.columns.indexOf(coordinateLayer.value.field) : -1
  const categoryIndex = coordinateLayer.kind === 'point' && coordinateLayer.category ? dataset.columns.indexOf(coordinateLayer.category.field) : -1
  const labelIndex = coordinateLayer.label ? dataset.columns.indexOf(coordinateLayer.label.field) : -1
  const features: Feature<Geometry>[] = []
  const selectableRows = envelope.dataState.kind !== 'spatial_windowed' || envelope.dataState.window?.precision === 'raw'
  for (let index = 0; index < dataset.rows.length; index++) {
    const row = dataset.rows[index]!
    const latitude = row[latitudeIndex], longitude = row[longitudeIndex]
    if (typeof latitude !== 'number' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90 || typeof longitude !== 'number' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180) continue
    features.push({ type: 'Feature', id: index, geometry: { type: 'Point', coordinates: [longitude, latitude] }, properties: {
      __ld_value: valueIndex >= 0 ? row[valueIndex] : 1,
      __ld_category: categoryIndex >= 0 ? row[categoryIndex] : null,
      __ld_label: labelIndex >= 0 ? String(row[labelIndex] ?? '') : '',
      __ld_selected: rowIsSelected(envelope, dataset.id, dataset.columns, row),
      __ld_has_selection: envelope.selection.length > 0,
      ...((layer.kind === 'point' || layer.tooltip.length > 0) && selectableRows ? rowLocator(dataset.id, index, layer.id) : {}),
    } })
  }
  return { type: 'FeatureCollection', features }
}

export function pathGeometry(envelope: VisualizationEnvelope, layer: Extract<VisualizationGeographicLayer, { kind: 'path' }>): FeatureCollection {
  const dataset = geographicDataset(envelope, layer.latitude.dataset)
  if (!dataset) return { type: 'FeatureCollection', features: [] }
  const latitudeIndex = dataset.columns.indexOf(layer.latitude.field), longitudeIndex = dataset.columns.indexOf(layer.longitude.field)
  const pathIndex = dataset.columns.indexOf(layer.path.field), orderIndex = dataset.columns.indexOf(layer.order.field)
  const valueIndex = layer.value ? dataset.columns.indexOf(layer.value.field) : -1
  const categoryIndex = layer.category ? dataset.columns.indexOf(layer.category.field) : -1
  const grouped = new Map<string, Array<{ coordinate: Position; order: unknown; value: unknown; category: unknown; rowIndex: number }>>()
  for (let rowIndex = 0; rowIndex < dataset.rows.length; rowIndex++) {
    const row = dataset.rows[rowIndex]!
    const latitude = row[latitudeIndex], longitude = row[longitudeIndex]
    if (typeof latitude !== 'number' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90 || typeof longitude !== 'number' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180) continue
    const key = String(row[pathIndex])
    const points = grouped.get(key) ?? []
    points.push({ coordinate: [longitude, latitude], order: row[orderIndex], value: valueIndex >= 0 ? row[valueIndex] : 1, category: categoryIndex >= 0 ? row[categoryIndex] : null, rowIndex })
    grouped.set(key, points)
  }
  const features: Feature<Geometry>[] = []
  const locatableRows = envelope.dataState.kind !== 'spatial_windowed' || envelope.dataState.window?.precision === 'raw'
  for (const [id, points] of grouped) {
    points.sort((a, b) => String(a.order).localeCompare(String(b.order), undefined, { numeric: true }))
    if (points.length < 2) continue
    const last = points.at(-1)!
    features.push({ type: 'Feature', id, geometry: { type: 'LineString', coordinates: points.map((point) => point.coordinate) }, properties: { __ld_value: last.value ?? 1, __ld_category: last.category ?? null, __ld_path: id, ...(locatableRows ? rowLocator(dataset.id, last.rowIndex, layer.id) : {}) } })
  }
  return { type: 'FeatureCollection', features }
}

export function mapLayer(id: string, layerOrKind: VisualizationGeographicLayer | VisualizationGeographicLayer['kind']): any {
  const layer = typeof layerOrKind === 'string' ? undefined : layerOrKind
  const kind = typeof layerOrKind === 'string' ? layerOrKind : layerOrKind.kind
  if (kind === 'choropleth') {
    const choropleth = layer?.kind === 'choropleth' ? layer : undefined
    return { id, source: id, type: 'fill', paint: { 'fill-color': ['case', ['==', ['get', '__ld_value'], null], choropleth?.color.nullColor ?? '#d8dee4', layerColorExpression(choropleth?.color)], 'fill-opacity': ['case', ['get', '__ld_selected'], 1, ['get', '__ld_has_selection'], 0.4, choropleth?.opacity ?? 0.82], 'fill-outline-color': choropleth?.stroke.color ?? '#ffffff' } }
  }
  if (kind === 'reference') {
    const reference = layer?.kind === 'reference' ? layer : undefined
    return { id, source: id, type: 'fill', filter: ['==', ['geometry-type'], 'Polygon'], paint: { 'fill-color': paletteColors(reference?.color)[2], 'fill-opacity': reference?.opacity ?? 0.18, 'fill-outline-color': reference?.stroke.color ?? '#57606a' } }
  }
  if (kind === 'path') {
    const path = layer?.kind === 'path' ? layer : undefined
    return { id, source: id, type: 'line', paint: { 'line-color': path?.category || path?.value ? layerColorExpression(path?.color) : path?.stroke.color ?? '#0969da', 'line-width': path?.line.width ?? 3, 'line-opacity': (path?.opacity ?? 0.82) * (path?.stroke.opacity ?? 1) } }
  }
  if (kind === 'point') {
    const point = layer?.kind === 'point' ? layer : undefined
    const minimumRadius = point?.size?.minimumRadius ?? 5, maximumRadius = point?.size?.maximumRadius ?? 10
    return { id, source: id, type: 'circle', filter: ['!', ['has', 'point_count']], minzoom: point?.visibility.minimumZoom, maxzoom: point?.visibility.maximumZoom, paint: { 'circle-radius': ['case', ['get', '__ld_selected'], maximumRadius + 3, ['interpolate', ['linear'], ['sqrt', ['get', '__ld_weight']], 0, minimumRadius, 1, maximumRadius]], 'circle-color': layerColorExpression(point?.color), 'circle-stroke-color': point?.stroke.color ?? '#ffffff', 'circle-stroke-opacity': point?.stroke.opacity ?? 1, 'circle-stroke-width': ['case', ['get', '__ld_selected'], (point?.stroke.width ?? 1.5) + 1, point?.stroke.width ?? 1.5], 'circle-opacity': ['case', ['get', '__ld_selected'], 1, ['get', '__ld_has_selection'], 0.3, point?.opacity ?? 0.78] } }
  }
  const heat = layer?.kind === 'heat' || layer?.kind === 'density' ? layer : undefined
  const colors = paletteColors(heat?.color)
  return { id, source: id, type: 'heatmap', paint: {
    'heatmap-weight': ['*', ['get', '__ld_weight'], ['case', ['get', '__ld_selected'], 1, 0.75]],
    'heatmap-intensity': heat?.heat.intensity ?? (kind === 'density' ? 1.35 : 1),
    'heatmap-radius': heat?.heat.radius ?? (kind === 'density' ? 24 : 32),
    'heatmap-opacity': heat?.opacity ?? 0.86,
    'heatmap-color': ['interpolate', ['linear'], ['heatmap-density'], 0, transparentColor(colors[0]), 0.15, colors[0], 0.35, colors[1], 0.6, colors[2], 0.85, colors[3], 1, colors[4]],
  } }
}

function colorInterpolation(scale?: { palette: string; reverse: boolean }): unknown[] {
  const colors = paletteColors(scale)
  return ['interpolate', ['linear'], ['get', '__ld_weight'], 0, colors[0], 0.25, colors[1], 0.5, colors[2], 0.75, colors[3], 1, colors[4]]
}

function layerColorExpression(scale?: { kind: string; palette: string; reverse: boolean; nullColor: string }): unknown[] {
  if (scale?.kind === 'categorical') return ['coalesce', ['get', '__ld_color'], scale.nullColor]
  return colorInterpolation(scale)
}

function paletteColors(scale?: { palette: string; reverse: boolean }): string[] {
  const palettes: Record<string, string[]> = {
    blue: ['#ddf4ff', '#80ccff', '#54aeff', '#0969da', '#0550ae'],
    teal: ['#e1f7f5', '#90e0d9', '#39c5bb', '#008c95', '#006d77'],
    purple: ['#fbefff', '#d8b9ff', '#bf87ff', '#8250df', '#6639ba'],
    orange: ['#fff1e5', '#ffc680', '#fb8f44', '#d15704', '#bc4c00'],
    red: ['#ffebe9', '#ffb3b6', '#ff8182', '#cf222e', '#a40e26'],
  }
  const selected = [...(palettes[scale?.palette ?? 'blue'] ?? palettes.blue!)]
  return scale?.reverse ? selected.reverse() : selected
}

function transparentColor(color: string): string {
  if (/^#[0-9a-f]{6}$/i.test(color)) return `${color}00`
  return 'rgba(9,105,218,0)'
}

function layerWeightDomain(layer: VisualizationGeographicLayer): { domainMinimum?: number; domainMidpoint?: number; domainMaximum?: number } | undefined {
  if (layer.kind === 'point' && layer.size && (layer.size.domainMinimum !== undefined || layer.size.domainMaximum !== undefined)) return layer.size
  if ('color' in layer) return layer.color
  return undefined
}

export function mapOutlineLayer(id: string, source: string): any {
  return {
    id, source, type: 'line',
    filter: ['==', ['get', '__ld_selected'], true],
    paint: { 'line-color': '#bf3989', 'line-opacity': 1, 'line-width': 3 },
  }
}

function rowLocator(datasetID: string, rowIndex: number, layerID: string): Record<string, string | number> {
  return { __ld_dataset: datasetID, __ld_row_index: rowIndex, __ld_layer_id: layerID }
}

type RenderedFeatureLocator = Readonly<{ layer?: { id?: string }; properties?: Record<string, unknown> | null }>

export function mapTooltipEntries(envelope: VisualizationEnvelope, features: readonly RenderedFeatureLocator[]): Array<{ label: string; value: string }> {
  if (envelope.spec.kind !== 'geographic') return []
  for (const feature of features) {
    const datasetID = feature.properties?.__ld_dataset, rowIndex = feature.properties?.__ld_row_index, layerID = feature.properties?.__ld_layer_id
    if (typeof datasetID !== 'string' || typeof rowIndex !== 'number' || !Number.isInteger(rowIndex) || rowIndex < 0 || typeof layerID !== 'string') continue
    const dataset = geographicDataset(envelope, datasetID)
    const layer = envelope.spec.layers.find((candidate) => candidate.id === layerID)
    const row = dataset?.rows[rowIndex]
    if (!dataset || !layer || !row) continue
    const fields = layer.tooltip.length ? layer.tooltip : layer.label ? [layer.label] : []
    return fields.flatMap((reference) => {
      if (reference.dataset !== datasetID) return []
      const column = dataset.columns.indexOf(reference.field)
      if (column < 0 || column >= row.length) return []
      const schema = envelope.spec.datasets.find((candidate) => candidate.id === datasetID)
      const field = schema?.fields.find((candidate) => candidate.id === reference.field)
      const raw = row[column]
      let value: string
      try { value = field?.format ? formatValue('en-US', field.format, raw) : raw == null ? '—' : String(raw) } catch { value = raw == null ? '—' : String(raw) }
      return [{ label: field?.label ?? reference.field, value }]
    })
  }
  return []
}

export function mapAccessibleData(envelope: VisualizationEnvelope, limit = 100): {
  columns: Array<{ id: string; label: string }>
  rows: string[][]
  totalRows: number
} {
  if (envelope.spec.kind !== 'geographic' || limit < 1) return { columns: [], rows: [], totalRows: 0 }
  const schema = envelope.spec.datasets[0]
  if (!schema) return { columns: [], rows: [], totalRows: 0 }
  const dataset = geographicDataset(envelope, schema.id)
  if (!dataset) return { columns: [], rows: [], totalRows: 0 }
  const fieldIDs: string[] = []
  const add = (reference?: { dataset: string; field: string }) => {
    if (reference?.dataset === schema.id && !fieldIDs.includes(reference.field)) fieldIDs.push(reference.field)
  }
  for (const layer of envelope.spec.layers) {
    for (const reference of layer.tooltip) add(reference)
    if (layer.tooltip.length > 0) continue
    add(layer.label)
    if (layer.kind === 'choropleth') { add(layer.join); add(layer.value); add(layer.category) }
    if (layer.kind === 'point') { add(layer.latitude); add(layer.longitude); add(layer.value); add(layer.category) }
    if (layer.kind === 'heat' || layer.kind === 'density') { add(layer.latitude); add(layer.longitude); add(layer.value) }
    if (layer.kind === 'path') { add(layer.path); add(layer.order); add(layer.latitude); add(layer.longitude); add(layer.value); add(layer.category) }
  }
  if (fieldIDs.length === 0) for (const field of schema.fields.slice(0, 3)) add({ dataset: schema.id, field: field.id })
  const columns = fieldIDs.flatMap((id) => {
    const field = schema.fields.find((candidate) => candidate.id === id)
    return field ? [{ id, label: field.label }] : []
  })
  const indexes = columns.map((column) => dataset.columns.indexOf(column.id))
  const fields = columns.map((column) => schema.fields.find((field) => field.id === column.id))
  const rows = dataset.rows.slice(0, Math.min(limit, dataset.rows.length)).map((row) => indexes.map((index, columnIndex) => {
    const raw = index >= 0 ? row[index] : null
    const field = fields[columnIndex]
    try { return field?.format ? formatValue('en-US', field.format, raw) : raw == null ? '—' : String(raw) } catch { return raw == null ? '—' : String(raw) }
  }))
  return { columns, rows, totalRows: dataset.rows.length }
}

function geographicDataset(envelope: VisualizationEnvelope, datasetID: string): { id: string; columns: string[]; rows: unknown[][] } | undefined {
  if (envelope.dataState.kind === 'inline') return envelope.dataState.datasets.find((candidate) => candidate.id === datasetID)
  if (envelope.dataState.kind === 'spatial_windowed' && envelope.dataState.schema.id === datasetID && envelope.dataState.window) {
    return { id: datasetID, columns: envelope.dataState.schema.fields.map((field) => field.id), rows: envelope.dataState.window.rows }
  }
  return undefined
}

export function spatialWindowRequest(
  envelope: VisualizationEnvelope,
  bounds: VisualizationSpatialBounds,
  zoom: number,
  width: number,
  height: number,
  requestSeq: number,
): MapSpatialWindowRequest | undefined {
  if (envelope.dataState.kind !== 'spatial_windowed') return undefined
  const values = [bounds.west, bounds.south, bounds.east, bounds.north, zoom]
  if (!values.every(Number.isFinite) || bounds.west < -180 || bounds.west > 180 || bounds.east < -180 || bounds.east > 180 || bounds.south < -90 || bounds.south > 90 || bounds.north < -90 || bounds.north > 90 || bounds.south > bounds.north) return undefined
  width = Math.max(1, Math.round(width)); height = Math.max(1, Math.round(height))
  const normalizedZoom = Math.max(0, Math.min(24, zoom))
  const windowID = `${bounds.west.toFixed(6)},${bounds.south.toFixed(6)},${bounds.east.toFixed(6)},${bounds.north.toFixed(6)}@${normalizedZoom.toFixed(3)}:${width}x${height}`
  return {
    visualID: envelope.visualID,
    specRevision: envelope.specRevision,
    dataRevision: envelope.dataRevision,
    requestSeq,
    resetVersion: envelope.dataState.resetVersion,
    bounds,
    zoom: normalizedZoom,
    width,
    height,
    windowID,
  }
}

export function interactionCommandForRenderedFeatures(
  envelope: VisualizationEnvelope,
  features: readonly RenderedFeatureLocator[],
  selectableLayerIDs: readonly string[],
) {
  const selectable = new Set(selectableLayerIDs)
  for (const feature of features) {
    const renderedLayerID = feature.layer?.id
    const datasetID = feature.properties?.__ld_dataset
    const rowIndex = feature.properties?.__ld_row_index
    const authoredLayerID = feature.properties?.__ld_layer_id
    if (typeof renderedLayerID !== 'string' || !selectable.has(renderedLayerID)) continue
    if (renderedLayerID !== `ld-${authoredLayerID}` || typeof datasetID !== 'string' || typeof rowIndex !== 'number') continue
    const command = interactionCommandForRowIndex(envelope, datasetID, rowIndex)
    if (command) return command
  }
  return undefined
}

export function mapInteractionCommand(
  envelope: VisualizationEnvelope,
  features: readonly RenderedFeatureLocator[],
  selectableLayerIDs: readonly string[],
): OptimisticInteractionCommand | undefined {
  return interactionCommandForRenderedFeatures(envelope, features, selectableLayerIDs)
    ?? (envelope.selection.length > 0 ? clearInteractionCommand(envelope) : undefined)
}

export function updateSelectionSources(
  envelope: VisualizationEnvelope,
  layers: readonly { spec: VisualizationGeographicLayer; sourceID: string; geometry?: FeatureCollection }[],
  getSource: (sourceID: string) => Pick<GeoJSONSource, 'setData'> | undefined,
): number {
  let updated = 0
  for (const layer of layers) {
    const data = layer.spec.kind === 'choropleth' && layer.geometry
      ? joinGeometry(envelope, layer.spec, layer.geometry)
      : layer.spec.kind === 'path'
        ? pathGeometry(envelope, layer.spec)
        : layer.spec.kind === 'reference' && layer.geometry
          ? layer.geometry
          : coordinateGeometry(envelope, layer.spec)
    const source = getSource(layer.sourceID)
    if (!source) continue
    source.setData(applyFeatureScales(data, layer.spec))
    updated++
  }
  return updated
}

export function applyBasemapTheme(map: Pick<MapLibreMap, 'getStyle' | 'getLayer' | 'setPaintProperty' | 'setLayoutProperty'>, colors: BasemapColors, background: string, labelDensity: 'hidden' | 'normal' | 'dense' = 'normal'): void {
  for (const layer of map.getStyle().layers ?? []) {
    if (!map.getLayer(layer.id)) continue
    const role = (layer.metadata as Record<string, unknown> | undefined)?.['libredash:role']
    if (role === 'background' && layer.type === 'background') map.setPaintProperty(layer.id, 'background-color', colors.background ?? background)
    if (role === 'land' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', colors.land)
    if (role === 'water' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', colors.water ?? '#cce8f7')
    if (role === 'water' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', colors.water ?? '#7bb9dc')
    if (role === 'boundary' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', colors.boundary)
    if (role === 'road' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', colors.road ?? '#ffffff')
    if (role === 'building' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', colors.building ?? '#d8dee4')
    if (role === 'label' && layer.type === 'symbol') {
      map.setLayoutProperty(layer.id, 'visibility', labelDensity === 'hidden' ? 'none' : 'visible')
      map.setPaintProperty(layer.id, 'text-color', colors.label ?? '#57606a')
      map.setPaintProperty(layer.id, 'text-halo-color', colors.land)
    }
  }
}

export function normalizeFeatureWeights(data: FeatureCollection, domain?: { domainMinimum?: number; domainMidpoint?: number; domainMaximum?: number }): FeatureCollection {
  const values = data.features.map((feature) => feature.properties?.__ld_value).filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  const minimum = domain?.domainMinimum ?? (values.length > 0 ? Math.min(...values) : 0)
  const maximum = domain?.domainMaximum ?? (values.length > 0 ? Math.max(...values) : 0)
  const span = maximum - minimum
  return {
    ...data,
    features: data.features.map((feature) => {
      const value = feature.properties?.__ld_value
      let weight = typeof value !== 'number' || !Number.isFinite(value) ? 0 : span === 0 ? (value === 0 ? 0 : 1) : Math.max(0, Math.min(1, (value - minimum) / span))
      const midpoint = domain?.domainMidpoint
      if (typeof value === 'number' && Number.isFinite(value) && midpoint !== undefined && midpoint > minimum && midpoint < maximum) {
        weight = value <= midpoint
          ? 0.5 * Math.max(0, Math.min(1, (value - minimum) / (midpoint - minimum)))
          : 0.5 + 0.5 * Math.max(0, Math.min(1, (value - midpoint) / (maximum - midpoint)))
      }
      return { ...feature, properties: { ...feature.properties, __ld_weight: weight } }
    }),
  }
}

export function applyFeatureScales(data: FeatureCollection, layer: VisualizationGeographicLayer): FeatureCollection {
  const normalized = normalizeFeatureWeights(data, layerWeightDomain(layer))
  if (!('color' in layer) || layer.color.kind !== 'categorical') return normalized
  const categories = [...new Set(normalized.features.map((feature) => feature.properties?.__ld_category).filter((value) => value !== null && value !== undefined).map(String))].sort((a, b) => a.localeCompare(b))
  const colors = paletteColors(layer.color)
  const colorByCategory = new Map(categories.map((category, index) => [category, colors[index % colors.length]!]))
  return {
    ...normalized,
    features: normalized.features.map((feature) => {
      const category = feature.properties?.__ld_category
      const color = category === null || category === undefined ? layer.color.nullColor : colorByCategory.get(String(category)) ?? layer.color.nullColor
      return { ...feature, properties: { ...feature.properties, __ld_color: color } }
    }),
  }
}

type GeographicViewport = {
  fitBounds(bounds: [[number, number], [number, number]], options: { padding: number; duration: number; maxZoom: number }): unknown
  jumpTo?(options: { center?: [number, number]; zoom?: number }): unknown
}

export function fitMapToGeographicData(map: GeographicViewport, collections: FeatureCollection[], camera?: VisualizationMapCamera): boolean {
  if (camera?.mode === 'preserve') return false
  if (camera?.mode === 'fixed' && camera.center && camera.center.length === 2) {
    map.jumpTo?.({ center: [camera.center[0]!, camera.center[1]!], zoom: camera.zoom })
    return true
  }
  const extent = geographicExtent(collections)
  if (!extent) return false
  let [[west, south], [east, north]] = extent
  if (west === east) { west -= 0.01; east += 0.01 }
  if (south === north) { south -= 0.01; north += 0.01 }
  map.fitBounds([[west, south], [east, north]], { padding: camera?.padding ?? 24, duration: 0, maxZoom: camera?.maximumZoom ?? 10 })
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
