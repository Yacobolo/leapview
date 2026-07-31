import rawContracts from '../../../../internal/dashboard/layout/contracts.json'

export type WidgetLayoutFeature =
  | 'subtitle'
  | 'comparison'
  | 'progress'
  | 'goal'
  | 'status'
  | 'trend'
  | 'note'

export type WidgetContractID =
  | 'kpi'
  | 'slicer.dropdown'
  | 'slicer.input'
  | 'slicer.numeric_range'
  | 'slicer.date_range'
  | 'slicer.relative_period'

export type WidgetSize = Readonly<{ width: number; height: number }>
export type WidgetLayoutRequirement = Readonly<{
  layout: string
  minimum: WidgetSize
}>
export type WidgetLayoutResolution =
  | Readonly<{ kind: 'fit'; layout: string; minimum: WidgetSize }>
  | Readonly<{ kind: 'too-small'; requirements: readonly WidgetLayoutRequirement[] }>

type FeatureCost = Readonly<{ width?: number; height?: number }>
type LayoutContract = Readonly<{
  id: string
  minimum: WidgetSize
  features: Readonly<Partial<Record<WidgetLayoutFeature, FeatureCost>>>
}>
type WidgetContract = Readonly<{
  layouts: readonly LayoutContract[]
  chrome: WidgetSize
}>

const contracts = parseContracts(rawContracts)

export function layoutRequirements(
  contractID: WidgetContractID,
  features: readonly WidgetLayoutFeature[] = [],
): readonly WidgetLayoutRequirement[] {
  const contract = contracts[contractID]
  const enabled = new Set(features)
  return contract.layouts.map((layout) => {
    let widthAddition = 0
    let height = layout.minimum.height
    for (const feature of enabled) {
      const cost = layout.features[feature]
      if (!cost) throw new Error(`layout ${JSON.stringify(layout.id)} does not support explicit feature ${JSON.stringify(feature)}`)
      widthAddition = Math.max(widthAddition, cost.width ?? 0)
      height += cost.height ?? 0
    }
    return { layout: layout.id, minimum: { width: layout.minimum.width + widthAddition, height } }
  })
}

export function resolveWidgetLayout(
  contractID: WidgetContractID,
  size: WidgetSize,
  features: readonly WidgetLayoutFeature[] = [],
): WidgetLayoutResolution {
  const requirements = layoutRequirements(contractID, features)
  const match = requirements.find(({ minimum }) => size.width >= minimum.width && size.height >= minimum.height)
  return match ? { kind: 'fit', ...match } : { kind: 'too-small', requirements }
}

export function widgetChrome(contractID: WidgetContractID): WidgetSize {
  return contracts[contractID].chrome
}

function parseContracts(value: unknown): Readonly<Record<WidgetContractID, WidgetContract>> {
  if (!isRecord(value) || value.version !== 1 || !isRecord(value.widgets)) {
    throw new Error('invalid dashboard layout contract')
  }
  const expected: WidgetContractID[] = [
    'kpi',
    'slicer.dropdown',
    'slicer.input',
    'slicer.numeric_range',
    'slicer.date_range',
    'slicer.relative_period',
  ]
  const parsed = {} as Record<WidgetContractID, WidgetContract>
  for (const id of expected) parsed[id] = parseWidget(id, value.widgets[id])
  return Object.freeze(parsed)
}

function parseWidget(id: string, value: unknown): WidgetContract {
  if (!isRecord(value) || !Array.isArray(value.layouts) || value.layouts.length === 0 || !isSize(value.chrome)) {
    throw new Error(`invalid dashboard layout contract for ${JSON.stringify(id)}`)
  }
  const seen = new Set<string>()
  const layouts = value.layouts.map((candidate): LayoutContract => {
    if (!isRecord(candidate) || typeof candidate.id !== 'string' || !candidate.id || seen.has(candidate.id) || !isSize(candidate.minimum) || !isRecord(candidate.features)) {
      throw new Error(`invalid dashboard layout in contract ${JSON.stringify(id)}`)
    }
    seen.add(candidate.id)
    const features: Partial<Record<WidgetLayoutFeature, FeatureCost>> = {}
    for (const [name, cost] of Object.entries(candidate.features)) {
      if (!isFeature(name) || !isRecord(cost)) throw new Error(`invalid feature cost ${JSON.stringify(name)} in contract ${JSON.stringify(id)}`)
      const width = optionalDimension(cost.width)
      const height = optionalDimension(cost.height)
      features[name] = Object.freeze({
        ...(width === undefined ? {} : { width }),
        ...(height === undefined ? {} : { height }),
      })
    }
    return Object.freeze({
      id: candidate.id,
      minimum: Object.freeze({ width: candidate.minimum.width, height: candidate.minimum.height }),
      features: Object.freeze(features),
    })
  })
  return Object.freeze({
    layouts: Object.freeze(layouts),
    chrome: Object.freeze({ width: value.chrome.width, height: value.chrome.height }),
  })
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isSize(value: unknown): value is { width: number; height: number } {
  return isRecord(value) && validDimension(value.width) && validDimension(value.height)
}

function validDimension(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

function optionalDimension(value: unknown): number | undefined {
  if (value === undefined) return undefined
  if (!validDimension(value)) throw new Error('layout feature dimensions must be finite and non-negative')
  return value
}

function isFeature(value: string): value is WidgetLayoutFeature {
  return ['subtitle', 'comparison', 'progress', 'goal', 'status', 'trend', 'note'].includes(value)
}
