import type { Map as MapLibreMap } from 'maplibre-gl'

export type BasemapColors = Readonly<{
  boundary: string
  land: string
  background?: string
  landcover?: string
  park?: string
  urban?: string
  water?: string
  waterway?: string
  boundarySecondary?: string
  road?: string
  roadHighway?: string
  roadMinor?: string
  roadCasing?: string
  building?: string
  label?: string
  labelRegion?: string
  labelLocality?: string
  labelSecondary?: string
  labelHalo?: string
}>

type FrameScheduler = (callback: () => void) => void

export function basemapThemeKey(colors: BasemapColors, background: string, labelDensity: 'hidden' | 'normal' | 'dense'): string {
  return [
    colors.background, colors.land, colors.landcover, colors.park, colors.urban,
    colors.water, colors.waterway, colors.boundary, colors.boundarySecondary,
    colors.road, colors.roadHighway, colors.roadMinor, colors.roadCasing,
    colors.building, colors.label, colors.labelRegion, colors.labelLocality,
    colors.labelSecondary, colors.labelHalo, background, labelDensity,
  ].join('\u0000')
}

// Updating several MapLibre styles synchronously can overwhelm the browser's
// shared WebGL process on map-heavy dashboards. Serialize style mutations one
// animation frame at a time while allowing each map to keep its own viewport.
export function createBasemapThemeScheduler(scheduleFrame: FrameScheduler): (update: () => void) => Promise<void> {
  let tail = Promise.resolve()
  return (update) => {
    const pending = tail.then(() => new Promise<void>((resolve, reject) => {
      scheduleFrame(() => {
        try {
          update()
          resolve()
        } catch (error) {
          reject(error)
        }
      })
    }))
    tail = pending.catch(() => {})
    return pending
  }
}

export const scheduleBasemapThemeMutation = createBasemapThemeScheduler((callback) => {
  if (typeof requestAnimationFrame === 'function' && document.visibilityState !== 'hidden') {
    requestAnimationFrame(() => callback())
    return
  }
  queueMicrotask(callback)
})

export function mapThemeColors(theme: 'auto' | 'light' | 'dark', resolved: 'light' | 'dark'): BasemapColors {
  const effective = theme === 'auto' ? resolved : theme
  if (effective === 'dark') return {
    background: '#0c1b26', land: '#18252d', landcover: '#1d2b31', park: '#1d322d', urban: '#222c33',
    water: '#0c1b26', waterway: '#294758', boundary: '#86939d', boundarySecondary: '#55636e',
    road: '#495864', roadHighway: '#725f49', roadMinor: '#34434e', roadCasing: '#111c24',
    building: '#293840', label: '#edf2f5', labelRegion: '#d2dbe1', labelLocality: '#c1ccd3',
    labelSecondary: '#9dabb4', labelHalo: '#17242c',
  }
  return {
    background: '#c9e2ec', land: '#f5f3ed', landcover: '#ebece3', park: '#e2eddd', urban: '#eeeae2',
    water: '#c9e2ec', waterway: '#93c2d4', boundary: '#737b82', boundarySecondary: '#aaa7a0',
    road: '#ffffff', roadHighway: '#e2b873', roadMinor: '#f5f3ed', roadCasing: '#d2cec4',
    building: '#dedbd3', label: '#30363b', labelRegion: '#484f55', labelLocality: '#596168',
    labelSecondary: '#737b82', labelHalo: '#f7f5ef',
  }
}

export function basemapLayer(id: string, colors: BasemapColors): any {
  return { id, source: id, type: 'fill', paint: { 'fill-color': colors.land, 'fill-opacity': 1 } }
}

export function basemapBoundaryLayer(id: string, source: string, boundary: string): any {
  return { id, source, type: 'line', paint: { 'line-color': boundary, 'line-opacity': 0.92, 'line-width': 1.5 } }
}

export function concreteCSSColor(resolved: string, fallback: string): string {
  return resolved.trim() || fallback
}

export function applyBasemapTheme(map: Pick<MapLibreMap, 'getStyle' | 'getLayer' | 'setPaintProperty' | 'setLayoutProperty'>, colors: BasemapColors, background: string, labelDensity: 'hidden' | 'normal' | 'dense' = 'normal'): void {
  for (const layer of map.getStyle().layers ?? []) {
    if (!map.getLayer(layer.id)) continue
    const role = basemapRole(layer)
    if (role === 'background' && layer.type === 'background') map.setPaintProperty(layer.id, 'background-color', colors.background ?? background)
    if (role === 'land' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', landColor(layer.id, colors))
    if (role === 'water' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', colors.water ?? '#cce8f7')
    if (role === 'water' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', colors.waterway ?? colors.water ?? '#7bb9dc')
    if (role === 'boundary' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', layer.id === 'boundaries_country' ? colors.boundary : colors.boundarySecondary ?? colors.boundary)
    if (role === 'road' && layer.type === 'line') map.setPaintProperty(layer.id, 'line-color', roadColor(layer.id, colors))
    if (role === 'building' && layer.type === 'fill') map.setPaintProperty(layer.id, 'fill-color', colors.building ?? '#d8dee4')
    if (role === 'label' && layer.type === 'symbol') {
      map.setLayoutProperty(layer.id, 'visibility', labelVisibility(layer.id, labelDensity))
      map.setPaintProperty(layer.id, 'text-color', labelColor(layer.id, colors))
      map.setPaintProperty(layer.id, 'text-halo-color', colors.labelHalo ?? colors.land)
    }
  }
}

function basemapRole(layer: { id: string; type: string; metadata?: unknown }): string | undefined {
  const declared = (layer.metadata as Record<string, unknown> | undefined)?.['leapview:role']
  if (typeof declared === 'string') return declared
  if (layer.type === 'background') return 'background'
  if (layer.type === 'symbol') return 'label'
  if (layer.id === 'earth' || layer.id === 'landcover' || layer.id.startsWith('landuse_')) return 'land'
  if (layer.id === 'water' || layer.id.startsWith('water_')) return 'water'
  if (layer.id.startsWith('boundaries')) return 'boundary'
  if (layer.id.startsWith('roads_')) return 'road'
  if (layer.id === 'buildings') return 'building'
  return undefined
}

function landColor(id: string, colors: BasemapColors): string {
  if (/park|urban_green|zoo/.test(id)) return colors.park ?? colors.land
  if (/industrial|hospital|school|aerodrome|runway/.test(id)) return colors.urban ?? colors.land
  if (id === 'landcover' || /beach|pedestrian|pier/.test(id)) return colors.landcover ?? colors.land
  return colors.land
}

function roadColor(id: string, colors: BasemapColors): string {
  if (id.includes('_casing')) return colors.roadCasing ?? colors.boundary
  if (id.includes('highway')) return colors.roadHighway ?? colors.road ?? '#ffffff'
  if (id.includes('major') || id.includes('_link')) return colors.road ?? '#ffffff'
  return colors.roadMinor ?? colors.road ?? '#ffffff'
}

function labelColor(id: string, colors: BasemapColors): string {
  if (id === 'places_country') return colors.label ?? '#30363b'
  if (id === 'places_region') return colors.labelRegion ?? colors.label ?? '#484f55'
  if (id === 'places_locality') return colors.labelLocality ?? colors.label ?? '#596168'
  return colors.labelSecondary ?? colors.label ?? '#737b82'
}

function labelVisibility(id: string, density: 'hidden' | 'normal' | 'dense'): 'none' | 'visible' {
  if (density === 'hidden') return 'none'
  if (density === 'dense') return 'visible'
  return /^(address_label|pois|places_subplace|roads_labels_minor)$/.test(id) ? 'none' : 'visible'
}
