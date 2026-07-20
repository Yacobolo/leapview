import { RendererRegistry } from './host-controller'

export const visualizationRegistry = new RendererRegistry()

visualizationRegistry.register({
  id: 'echarts', version: '6.1.0', schemaVersions: [1], kinds: ['cartesian', 'proportional', 'hierarchy', 'polar'],
  capabilities: { snapshot: true, windowed: false, interactive: true },
  load: async () => (await import('./adapters/echarts')).adapter,
})
visualizationRegistry.register({
  id: 'html', version: '1.0.0', schemaVersions: [1], kinds: ['kpi'],
  capabilities: { snapshot: true, windowed: false, interactive: true },
  load: async () => (await import('./adapters/html')).adapter,
})
visualizationRegistry.register({
  id: 'tanstack', version: '9.0.0-beta.12', schemaVersions: [1], kinds: ['table', 'matrix', 'pivot'],
  capabilities: { snapshot: true, windowed: true, interactive: true },
  load: async () => (await import('./adapters/tanstack')).adapter,
})
visualizationRegistry.register({
  id: 'maplibre', version: '5.19.0', schemaVersions: [1], kinds: ['geographic'],
  capabilities: { snapshot: true, windowed: false, interactive: true },
  load: async () => (await import('./adapters/maplibre')).adapter,
})
visualizationRegistry.register({
  id: 'vega-lite-sandbox', version: '6.4.3', schemaVersions: [1], kinds: ['custom'],
  capabilities: { snapshot: true, windowed: false, interactive: true },
  load: async () => (await import('./adapters/vega-lite')).adapter,
})
