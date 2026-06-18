import { LitElement, html } from 'lit'
import { property } from 'lit/decorators.js'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import '@xyflow/react/dist/style.css'
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
} from '@xyflow/react'

type LineageGraph = {
  nodes: LineageNode[]
  edges: LineageEdge[]
}

type LineageNode = {
  id: string
  label: string
  kind: string
  meta?: string
  href?: string
  side?: 'upstream' | 'selected' | 'downstream'
  selected?: boolean
}

type LineageEdge = {
  id: string
  source: string
  target: string
  label?: string
  kind: string
}

class AssetLineageGraph extends LitElement {
  @property({ type: Object }) graph: LineageGraph | null = null
  @property({ attribute: 'data-graph' }) dataGraph = '{}'
  private root?: Root
  private mount?: HTMLDivElement

  createRenderRoot(): HTMLElement {
    return this
  }

  firstUpdated(): void {
    this.mount = this.renderRoot.querySelector('.asset-lineage-flow') as HTMLDivElement | null ?? undefined
    if (this.mount) {
      this.root = createRoot(this.mount)
      this.renderFlow()
    }
  }

  updated(changed: Map<string, unknown>): void {
    if (changed.has('graph') || changed.has('dataGraph')) this.renderFlow()
  }

  disconnectedCallback(): void {
    this.root?.unmount()
    super.disconnectedCallback()
  }

  render() {
    return html`
      <style>
        ${assetLineageGraphStyles}
      </style>
      <div class="asset-lineage-flow" aria-label="Asset lineage graph"></div>
    `
  }

  private renderFlow(): void {
    if (!this.root) return
    const graph = this.resolvedGraph
    this.root.render(
      React.createElement(ReactFlow, {
        nodes: graph.nodes.map((node) => toFlowNode(node, graph.nodes)),
        edges: graph.edges.map(toFlowEdge),
        nodeTypes: { lineageNode: LineageNodeComponent },
        fitView: true,
        fitViewOptions: { padding: 0.2 },
        minZoom: 0.5,
        maxZoom: 1.35,
        nodesDraggable: false,
        nodesConnectable: false,
        elementsSelectable: false,
        panOnDrag: true,
        zoomOnScroll: false,
        preventScrolling: false,
        children: [
          React.createElement(Background, { key: 'background', gap: 18, size: 1 }),
          React.createElement(Controls, { key: 'controls', showInteractive: false }),
        ],
      }),
    )
  }

  private get resolvedGraph(): LineageGraph {
    if (this.graph) {
      return {
        nodes: this.graph.nodes ?? [],
        edges: this.graph.edges ?? [],
      }
    }
    try {
      const parsed = JSON.parse(this.dataGraph) as LineageGraph
      return {
        nodes: parsed.nodes ?? [],
        edges: parsed.edges ?? [],
      }
    } catch {
      return { nodes: [], edges: [] }
    }
  }
}

const assetLineageGraphStyles = `
  ld-asset-lineage-graph .asset-lineage-flow {
    height: 100%;
    min-height: 0;
    min-width: 0;
    background:
      linear-gradient(var(--bgColor-default), var(--bgColor-default)),
      radial-gradient(circle at 1px 1px, color-mix(in srgb, var(--fgColor-muted), transparent 87%) 1px, transparent 0);
    background-size: auto, 18px 18px;
  }

  ld-asset-lineage-graph .react-flow {
    color: var(--fgColor-default);
  }

  ld-asset-lineage-graph .react-flow__attribution {
    display: none;
  }

  ld-asset-lineage-graph .react-flow__controls {
    border: var(--ld-border-default);
    background: var(--bgColor-default);
    box-shadow: var(--shadow-resting-small);
  }

  ld-asset-lineage-graph .react-flow__controls-button {
    border-bottom-color: var(--borderColor-muted);
    background: var(--bgColor-default);
    color: var(--fgColor-default);
  }

  ld-asset-lineage-graph .asset-lineage-node {
    width: 200px;
    border: 1px solid var(--lineage-node-border);
    border-left: 4px solid var(--lineage-node-accent);
    border-radius: var(--borderRadius-default);
    background: var(--lineage-node-bg);
    box-shadow: var(--shadow-resting-small);
    color: var(--fgColor-default);
    padding: 9px 10px;
  }

  ld-asset-lineage-graph .asset-lineage-node-selected {
    border-color: var(--borderColor-accent-emphasis);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--borderColor-accent-emphasis), transparent 28%), var(--shadow-resting-small);
  }

  ld-asset-lineage-graph .asset-lineage-node-kind {
    color: var(--fgColor-muted);
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-950);
    text-transform: uppercase;
  }

  ld-asset-lineage-graph .asset-lineage-node-title {
    display: block;
    overflow: hidden;
    margin-top: 3px;
    color: var(--fgColor-default);
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--ld-font-size-body-md);
    font-weight: var(--ld-font-weight-900);
    line-height: var(--ld-line-height-tight);
    text-decoration: none;
  }

  ld-asset-lineage-graph .asset-lineage-node-title[href]:hover,
  ld-asset-lineage-graph .asset-lineage-node-title[href]:focus-visible {
    color: var(--fgColor-accent);
    outline: 0;
    text-decoration: underline;
  }

  ld-asset-lineage-graph .asset-lineage-node-meta {
    overflow: hidden;
    margin-top: 5px;
    color: var(--fgColor-muted);
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--ld-font-size-caption);
    font-weight: var(--ld-font-weight-750);
  }
`

function toFlowNode(node: LineageNode, nodes: LineageNode[]): Node {
  const { x, y } = positionFor(node, nodes)
  return {
    id: node.id,
    type: 'lineageNode',
    position: { x, y },
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    data: node,
  }
}

function toFlowEdge(edge: LineageEdge): Edge {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    label: edge.label ?? '',
    markerEnd: { type: MarkerType.ArrowClosed },
    style: {
      stroke: edgeStroke(edge.kind),
      strokeWidth: 1.5,
    },
    labelStyle: {
      fill: 'var(--fgColor-muted)',
      fontSize: 10,
      fontWeight: 800,
    },
    labelBgStyle: {
      fill: 'var(--bgColor-default)',
      fillOpacity: 0.92,
    },
  }
}

function positionFor(node: LineageNode, nodes: LineageNode[]): { x: number; y: number } {
  const side = node.side ?? 'downstream'
  const sideNodes = nodes.filter((candidate) => (candidate.side ?? 'downstream') === side)
  const index = Math.max(0, sideNodes.findIndex((candidate) => candidate.id === node.id))
  const y = Math.max(16, index * 118)
  switch (side) {
    case 'upstream':
      return { x: 0, y }
    case 'selected':
      return { x: 280, y: Math.max(64, y) }
    default:
      return { x: 560, y }
  }
}

function LineageNodeComponent({ data }: { data: LineageNode }) {
  const styles = nodeStyle(data)
  const className = data.selected ? 'asset-lineage-node asset-lineage-node-selected' : 'asset-lineage-node'
  return React.createElement(
    'div',
    { className, style: styles },
    React.createElement(Handle, { type: 'target', position: Position.Left }),
    React.createElement('div', { className: 'asset-lineage-node-kind' }, kindLabel(data.kind)),
    data.href
      ? React.createElement('a', { className: 'asset-lineage-node-title', href: data.href, title: data.label }, data.label)
      : React.createElement('div', { className: 'asset-lineage-node-title', title: data.label }, data.label),
    data.meta ? React.createElement('div', { className: 'asset-lineage-node-meta' }, data.meta) : null,
    React.createElement(Handle, { type: 'source', position: Position.Right }),
  )
}

function nodeStyle(node: LineageNode): Record<string, string> {
  const palette: Record<string, [string, string, string]> = {
    cache_table: ['var(--data-auburn-color-muted)', 'var(--data-auburn-color-emphasis)', 'var(--borderColor-attention-muted)'],
    catalog: ['var(--data-pink-color-muted)', 'var(--data-pink-color-emphasis)', 'var(--borderColor-accent-muted)'],
    connection: ['var(--data-gray-color-muted)', 'var(--data-gray-color-emphasis)', 'var(--borderColor-muted)'],
    dashboard: ['var(--data-purple-color-muted)', 'var(--data-purple-color-emphasis)', 'var(--borderColor-accent-muted)'],
    dataset: ['var(--data-auburn-color-muted)', 'var(--data-auburn-color-emphasis)', 'var(--borderColor-attention-muted)'],
    dimension: ['var(--data-lemon-color-muted)', 'var(--data-lemon-color-emphasis)', 'var(--borderColor-attention-muted)'],
    filter: ['var(--data-pine-color-muted)', 'var(--data-pine-color-emphasis)', 'var(--borderColor-success-muted)'],
    measure: ['var(--data-green-color-muted)', 'var(--data-green-color-emphasis)', 'var(--borderColor-success-muted)'],
    metric_view: ['var(--data-yellow-color-muted)', 'var(--data-yellow-color-emphasis)', 'var(--borderColor-attention-muted)'],
    page: ['var(--data-coral-color-muted)', 'var(--data-coral-color-emphasis)', 'var(--borderColor-danger-muted)'],
    semantic_model: ['var(--data-blue-color-muted)', 'var(--data-blue-color-emphasis)', 'var(--borderColor-accent-muted)'],
    source: ['var(--data-teal-color-muted)', 'var(--data-teal-color-emphasis)', 'var(--borderColor-accent-muted)'],
    table: ['var(--data-olive-color-muted)', 'var(--data-olive-color-emphasis)', 'var(--borderColor-success-muted)'],
    visual: ['var(--data-red-color-muted)', 'var(--data-red-color-emphasis)', 'var(--borderColor-danger-muted)'],
  }
  const [bg, accent, border] = palette[node.kind] ?? palette.semantic_model
  return {
    '--lineage-node-bg': bg,
    '--lineage-node-accent': node.selected ? 'var(--borderColor-accent-emphasis)' : accent,
    '--lineage-node-border': border,
  } as Record<string, string>
}

function edgeStroke(kind: string): string {
  if (kind.startsWith('uses')) return 'var(--borderColor-accent-emphasis)'
  if (kind.startsWith('reads')) return 'var(--fgColor-attention)'
  if (kind.startsWith('filters')) return 'var(--fgColor-success)'
  return 'var(--borderColor-muted)'
}

function kindLabel(kind: string): string {
  switch (kind) {
    case 'cache_table':
      return 'Cache table'
    case 'metric_view':
      return 'Metric view'
    case 'semantic_model':
      return 'Semantic model'
    default:
      return kind.replaceAll('_', ' ').replace(/\b\w/g, (char) => char.toUpperCase())
  }
}

customElements.define('ld-asset-lineage-graph', AssetLineageGraph)
