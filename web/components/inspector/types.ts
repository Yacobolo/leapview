/**
 * Type definitions for Datastar Inspector
 */

/**
 * Persisted inspector state (stored in sessionStorage)
 */
export interface InspectorState {
  /** Whether the panel is expanded */
  expanded: boolean
  /** Current filter text */
  filter: string
  /** Expanded tree paths */
  expandedPaths?: string[]
  /** Active inspector surface. */
  view?: 'signals' | 'events'
}

/**
 * Signal data structure (recursive key-value)
 */
export type SignalValue = string | number | boolean | null | SignalValue[] | SignalObject

export interface SignalObject {
  [key: string]: SignalValue
}

export type PageStreamTraceStage = 'published' | 'coalesced' | 'dropped' | 'delivered'

export interface PageStreamTraceEvent {
  id: number
  timestamp: string
  streamId: string
  sequence: number
  stage: PageStreamTraceStage
  generation?: number
  origin?: string
  correlationId?: string
  roots: string[]
  bytes: number
  digest?: string
  queueMilliseconds?: number
  coalesced?: number
  outcome?: string
  payload?: Record<string, unknown>
}

export interface PageStreamTraceResponse {
  events: PageStreamTraceEvent[]
  nextAfter: number
}
