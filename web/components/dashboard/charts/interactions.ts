import type { ChartDatum, ChartPayload } from './types'
import {
  interactionSelectionLabel,
  interactionSelectionValue,
  type InteractionSelectionMapping,
} from '../interaction-selection'

export type ChartInteractionDetail = {
  sourceKind: 'visual'
  sourceId: string
  interactionKind: string
  action: 'set'
  toggle: boolean
  mappings: Array<InteractionSelectionMapping & { label: string }>
}

export function chartInteractionDetailForDatum(payload: ChartPayload, datum: ChartDatum): ChartInteractionDetail | undefined {
  const interaction = payload.interaction
  const mappings = interaction?.mappings ?? []
  if (!payload.id || mappings.length === 0) return undefined
  const commandMappings = mappings.map((mapping) => {
    const value = interactionSelectionValue(datum[mapping.value])
    if (value === undefined) return undefined
    const configuredLabel = mapping.label ? interactionSelectionValue(datum[mapping.label]) : undefined
    return {
      field: mapping.field,
      ...(mapping.fact !== undefined ? { fact: mapping.fact } : {}),
      ...(mapping.grain !== undefined ? { grain: mapping.grain } : {}),
      value,
      label: interactionSelectionLabel(configuredLabel === undefined ? value : configuredLabel),
    }
  })
  if (commandMappings.some((mapping) => mapping === undefined)) return undefined
  return {
    sourceKind: 'visual',
    sourceId: payload.id,
    interactionKind: interaction?.kind || 'point_selection',
    action: 'set',
    toggle: interaction?.toggle !== false,
    mappings: commandMappings as Array<InteractionSelectionMapping & { label: string }>,
  }
}
