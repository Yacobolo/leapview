export type Effect = () => void
export type JSONPatch = Record<string, unknown>
export type MergePatchArgs = { ifMissing?: boolean }
export type Paths = [string, unknown][]

export {
  actions,
  effect,
  mergePatch,
  mergePaths,
  root,
} from '/static/vendor/datastar-1.0.2.js?v=dev'
