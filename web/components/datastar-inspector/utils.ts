/**
 * Utility functions for Datastar Inspector
 */

import type { SignalObject } from './types.js'

export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function countSignals(obj: unknown, count = 0): number {
  if (typeof obj !== 'object' || obj === null) return count + 1
  for (const value of Object.values(obj as Record<string, unknown>)) {
    count = countSignals(value, count)
  }
  return count
}

export function flattenSignals(
  obj: Record<string, unknown>,
  prefix = ''
): Array<[string, unknown]> {
  const result: Array<[string, unknown]> = []

  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key

    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      result.push(...flattenSignals(value as Record<string, unknown>, path))
    } else {
      result.push([path, value])
    }
  }

  return result
}

export function parseFilterPattern(filterText: string): RegExp {
  if (filterText.startsWith('/') && filterText.lastIndexOf('/') > 0) {
    const lastSlash = filterText.lastIndexOf('/')
    const pattern = filterText.slice(1, lastSlash)
    const flags = filterText.slice(lastSlash + 1)
    try {
      return new RegExp(pattern, flags || 'i')
    } catch {
      return new RegExp(escapeRegex(filterText), 'i')
    }
  }

  if (filterText.includes('*')) {
    const pattern = escapeRegex(filterText).replace(/\\\*/g, '.*')
    return new RegExp(pattern, 'i')
  }

  return new RegExp(escapeRegex(filterText), 'i')
}

export function filterObject(
  obj: Record<string, unknown>,
  regex: RegExp,
  path = ''
): Record<string, unknown> {
  const result: Record<string, unknown> = {}

  for (const [key, value] of Object.entries(obj)) {
    const fullPath = path ? `${path}.${key}` : key

    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      const filtered = filterObject(value as Record<string, unknown>, regex, fullPath)
      if (Object.keys(filtered).length > 0) {
        result[key] = filtered
      }
    } else if (regex.test(fullPath) || regex.test(String(value))) {
      result[key] = value
    }
  }

  return result
}

export function findChangedPaths(
  oldObj: SignalObject,
  newObj: SignalObject,
  prefix = ''
): Set<string> {
  const changed = new Set<string>()

  for (const [key, newValue] of Object.entries(newObj)) {
    const path = prefix ? `${prefix}.${key}` : key
    const oldValue = oldObj[key]

    if (typeof newValue === 'object' && newValue !== null && !Array.isArray(newValue)) {
      if (typeof oldValue === 'object' && oldValue !== null && !Array.isArray(oldValue)) {
        const nestedChanged = findChangedPaths(
          oldValue as SignalObject,
          newValue as SignalObject,
          path
        )
        nestedChanged.forEach((p) => changed.add(p))
      } else {
        changed.add(path)
      }
    } else if (JSON.stringify(oldValue) !== JSON.stringify(newValue)) {
      changed.add(path)
    }
  }

  for (const key of Object.keys(oldObj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (!(key in newObj)) {
      changed.add(path)
    }
  }

  return changed
}

export function renderJsonValue(
  value: unknown,
  changedPaths: Set<string>,
  indent = 0,
  path = ''
): string {
  const pad = '  '.repeat(indent)

  if (value === null) {
    return `<span class="text-on-surface-variant">null</span>`
  }
  if (typeof value === 'boolean') {
    return `<span class="text-primary">${value}</span>`
  }
  if (typeof value === 'number') {
    return `<span class="text-warning">${value}</span>`
  }
  if (typeof value === 'string') {
    return `<span class="text-success">"${escapeHtml(value)}"</span>`
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return '[]'
    const items = value
      .map((v, i) => {
        const itemPath = `${path}[${i}]`
        return `${pad}  ${renderJsonValue(v, changedPaths, indent + 1, itemPath)}`
      })
      .join(',\n')
    return `[\n${items}\n${pad}]`
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value)
    if (entries.length === 0) return '{}'
    const items = entries
      .map(([k, v]) => {
        const keyPath = path ? `${path}.${k}` : k
        const isChanged = changedPaths.has(keyPath)
        const flashClass = isChanged ? ' bg-warning rounded px-1' : ''
        const lineContent = `<span class="text-secondary">"${escapeHtml(k)}"</span>: ${renderJsonValue(v, changedPaths, indent + 1, keyPath)}`
        return `${pad}  <span class="${flashClass.trim()}">${lineContent}</span>`
      })
      .join(',\n')
    return `{\n${items}\n${pad}}`
  }
  return String(value)
}
