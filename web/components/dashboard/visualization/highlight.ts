import type {
  VisualizationDatasetSchema,
  VisualizationEnvelope,
  VisualizationHighlightState,
} from '../../../generated/visualization'

export interface VisualizationHighlightProjection {
  active: boolean
  matchedRows: ReadonlySet<number>
  announcement: string
}

export function projectVisualizationHighlights(
  envelope: VisualizationEnvelope,
  datasetID: string,
  columns: readonly string[],
  rows: readonly (readonly unknown[])[],
): VisualizationHighlightProjection {
  const highlights = envelope.highlights ?? []
  if (highlights.length === 0) return { active: false, matchedRows: new Set(), announcement: '' }
  const schema = envelope.spec.datasets.find((candidate) => candidate.id === datasetID)
  const matchedRows = new Set<number>()
  if (schema) {
    rows.forEach((row, index) => {
      if (highlights.some((highlight) => highlightMatchesRow(highlight, schema, columns, row))) matchedRows.add(index)
    })
  }
  const labels = highlights.map((highlight) => highlight.label).filter(Boolean)
  const announcement = labels.length === 1
    ? `Highlighted selection: ${labels[0]}. Comparison totals are unchanged.`
    : `${labels.length} highlighted selections. Comparison totals are unchanged.`
  return { active: true, matchedRows, announcement }
}

function highlightMatchesRow(
  highlight: VisualizationHighlightState,
  schema: VisualizationDatasetSchema,
  columns: readonly string[],
  row: readonly unknown[],
): boolean {
  if (highlight.spatialGeometry && highlight.spatialLatitudeFieldID && highlight.spatialLongitudeFieldID) {
    const latitude = rowValue(schema, columns, row, highlight.spatialLatitudeFieldID)
    const longitude = rowValue(schema, columns, row, highlight.spatialLongitudeFieldID)
    if (typeof latitude === 'number' && Number.isFinite(latitude) && typeof longitude === 'number' && Number.isFinite(longitude)) {
      return spatialGeometryContains(highlight.spatialGeometry, longitude, latitude)
    }
  }
  return highlight.entries.some((entry) => entry.mappings.every((mapping) => {
    const field = highlightField(schema, mapping.targetFieldID)
    if (!field) return false
    const index = columns.indexOf(field.id)
    return index >= 0 && scalarEqual(row[index], mapping.value)
  }))
}

function rowValue(
  schema: VisualizationDatasetSchema,
  columns: readonly string[],
  row: readonly unknown[],
  targetFieldID: string,
): unknown {
  const field = highlightField(schema, targetFieldID)
  return field ? row[columns.indexOf(field.id)] : undefined
}

function highlightField(schema: VisualizationDatasetSchema, targetFieldID: string) {
  return schema.fields.find((candidate) =>
    candidate.id === targetFieldID
    || candidate.sourceRef === targetFieldID
    || candidate.provenance?.sourceRefs.includes(targetFieldID))
}

function spatialGeometryContains(
  geometry: VisualizationHighlightState['spatialGeometry'] & {},
  longitude: number,
  latitude: number,
): boolean {
  if (geometry.kind === 'box') {
    const insideLongitude = geometry.bounds.west <= geometry.bounds.east
      ? longitude >= geometry.bounds.west && longitude <= geometry.bounds.east
      : longitude >= geometry.bounds.west || longitude <= geometry.bounds.east
    return insideLongitude && latitude >= geometry.bounds.south && latitude <= geometry.bounds.north
  }
  if (geometry.kind === 'radius') {
    return haversineMeters(longitude, latitude, geometry.center.longitude, geometry.center.latitude) <= geometry.radiusMeters
  }
  let inside = false
  const points = geometry.points
  for (let index = 0, previous = points.length - 1; index < points.length; previous = index++) {
    const current = points[index]!, prior = points[previous]!
    const crosses = (current.latitude > latitude) !== (prior.latitude > latitude)
      && longitude < (prior.longitude - current.longitude) * (latitude - current.latitude)
        / (prior.latitude - current.latitude) + current.longitude
    if (crosses) inside = !inside
  }
  return inside
}

function haversineMeters(longitude: number, latitude: number, centerLongitude: number, centerLatitude: number): number {
  const radians = Math.PI / 180
  const latitudeDelta = (centerLatitude - latitude) * radians
  const longitudeDelta = (centerLongitude - longitude) * radians
  const a = Math.sin(latitudeDelta / 2) ** 2
    + Math.cos(latitude * radians) * Math.cos(centerLatitude * radians) * Math.sin(longitudeDelta / 2) ** 2
  return 2 * 6_371_008.8 * Math.asin(Math.min(1, Math.sqrt(a)))
}

function scalarEqual(left: unknown, right: unknown): boolean {
  return left === right || (typeof left === 'number' && typeof right === 'number' && Number.isFinite(left) && Number.isFinite(right) && left === right)
}
