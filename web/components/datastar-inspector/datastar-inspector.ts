/**
 * Datastar Inspector - Dev-only debugging tool
 *
 * A self-contained web component for inspecting Datastar signals.
 * Works in any Datastar project with zero configuration.
 */
import { LitElement, html, nothing } from 'lit'
import { customElement, state } from 'lit/decorators.js'

import type { InspectorState, ViewMode, SignalObject } from './types.js'
import {
  countSignals,
  flattenSignals,
  filterObject,
  findChangedPaths,
  parseFilterPattern,
  renderJsonValue,
} from './utils.js'

const FLASH_DURATION = 400
const STORAGE_KEY = 'ds-inspector'

@customElement('datastar-inspector')
export class DatastarInspector extends LitElement {
  override createRenderRoot() {
    return this
  }

  @state() private expanded = false
  @state() private filter = ''
  @state() private viewMode: ViewMode = 'json'
  @state() private signals: SignalObject = {}
  @state() private signalCount = 0
  @state() private changedPaths: Set<string> = new Set()
  @state() private hasUnseenChanges = false

  private observer: MutationObserver | null = null
  private signalsElementId = `ds-inspector-signals-${Math.random().toString(36).slice(2, 9)}`
  private previousSignals: SignalObject = {}
  private flashTimeout: number | null = null

  override connectedCallback() {
    super.connectedCallback()
    this.loadState()
  }

  override disconnectedCallback() {
    super.disconnectedCallback()
    this.observer?.disconnect()
    if (this.flashTimeout) {
      clearTimeout(this.flashTimeout)
    }
  }

  override firstUpdated() {
    this.setupSignalObserver()
  }

  private loadState() {
    try {
      const saved = sessionStorage.getItem(STORAGE_KEY)
      if (saved) {
        const state: InspectorState = JSON.parse(saved)
        this.expanded = state.expanded ?? false
        this.filter = state.filter ?? ''
        this.viewMode = state.viewMode ?? 'json'
      }
    } catch {
      /* ignore parse errors */
    }
  }

  private saveState() {
    const state: InspectorState = {
      expanded: this.expanded,
      filter: this.filter,
      viewMode: this.viewMode,
    }
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  }

  private setupSignalObserver() {
    const el = document.getElementById(this.signalsElementId)
    if (!el) return

    this.parseSignals(el.textContent || '{}', true)

    this.observer = new MutationObserver(() => {
      this.parseSignals(el.textContent || '{}', false)
    })
    this.observer.observe(el, { childList: true, characterData: true, subtree: true })
  }

  private parseSignals(json: string, isInitial: boolean) {
    try {
      const newSignals = JSON.parse(json) as SignalObject

      if (!isInitial && Object.keys(this.previousSignals).length > 0) {
        const changed = findChangedPaths(this.previousSignals, newSignals)
        if (changed.size > 0) {
          this.changedPaths = changed

          if (!this.expanded) {
            this.hasUnseenChanges = true
          }

          if (this.flashTimeout) {
            clearTimeout(this.flashTimeout)
          }
          this.flashTimeout = window.setTimeout(() => {
            this.changedPaths = new Set()
            this.hasUnseenChanges = false
          }, FLASH_DURATION)
        }
      }

      this.previousSignals = JSON.parse(json) as SignalObject
      this.signals = newSignals
      this.signalCount = countSignals(this.signals)
    } catch {
      this.signals = {}
      this.signalCount = 0
    }
  }

  private getFilteredSignals(): SignalObject {
    if (!this.filter.trim()) return this.signals

    const regex = parseFilterPattern(this.filter.trim())
    return filterObject(this.signals, regex) as SignalObject
  }

  private toggle() {
    this.expanded = !this.expanded
    this.saveState()
    if (this.expanded) {
      this.hasUnseenChanges = false
      requestAnimationFrame(() => this.setupSignalObserver())
    }
  }

  private close() {
    this.expanded = false
    this.saveState()
  }

  private handleFilterInput(e: Event) {
    this.filter = (e.target as HTMLInputElement).value
    this.saveState()
  }

  private clearFilter() {
    this.filter = ''
    this.saveState()
  }

  private setViewMode(mode: ViewMode) {
    this.viewMode = mode
    this.saveState()
  }

  override render() {
    const filteredSignals = this.getFilteredSignals()
    const filteredCount = countSignals(filteredSignals)
    const hasFilter = this.filter.trim().length > 0

    return html`
      <pre id="${this.signalsElementId}" class="hidden" data-json-signals></pre>

      ${this.expanded ? this.renderPanel(filteredSignals, filteredCount, hasFilter) : this.renderToggle()}
    `
  }

  private renderToggle() {
    return html`
      <button
        class="btn bg-primary text-on-primary border-primary btn-circle btn-sm fixed bottom-4 right-4 z-[99999] shadow-xl ${this.hasUnseenChanges ? 'animate-pulse' : ''}"
        @click=${this.toggle}
        title="Open Datastar Inspector"
      >
        DS
      </button>
    `
  }

  private renderPanel(filteredSignals: SignalObject, filteredCount: number, hasFilter: boolean) {
    return html`
      <div class="fixed bottom-4 right-4 z-[99999] flex h-[32rem] w-96 max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-box border border-outline-variant bg-surface shadow-2xl">
        ${this.renderHeader(filteredCount, hasFilter)}
        ${this.renderContent(filteredSignals, hasFilter)}
      </div>
    `
  }

  private renderHeader(filteredCount: number, hasFilter: boolean) {
    const placeholder = hasFilter
      ? `${filteredCount}/${this.signalCount} match...`
      : `Filter ${this.signalCount} signals...`

    return html`
      <div class="flex items-center gap-2 border-b border-outline-variant bg-container-low px-3 py-2">
        <span class="badge badge-primary badge-sm">DS</span>
        <input
          type="text"
          class="input input-bordered input-xs w-full"
          placeholder="${placeholder}"
          .value=${this.filter}
          @input=${this.handleFilterInput}
        />
        ${hasFilter
          ? html`<button class="btn btn-ghost btn-xs" @click=${this.clearFilter} title="Clear filter">&times;</button>`
          : nothing}
        <div class="join">
          <button
            class="btn btn-xs join-item ${this.viewMode === 'json' ? 'btn-active' : ''}"
            @click=${() => this.setViewMode('json')}
            title="JSON view"
          >
            { }
          </button>
          <button
            class="btn btn-xs join-item ${this.viewMode === 'table' ? 'btn-active' : ''}"
            @click=${() => this.setViewMode('table')}
            title="Table view"
          >
            ≡
          </button>
        </div>
        <button class="btn btn-ghost btn-xs" @click=${this.close} title="Close">&times;</button>
      </div>
    `
  }

  private renderContent(filteredSignals: SignalObject, hasFilter: boolean) {
    const isEmpty = Object.keys(filteredSignals).length === 0

    return html`
      <div class="flex-1 overflow-auto p-3">
        ${isEmpty
          ? html`<div class="flex h-full items-center justify-center text-sm text-on-surface-variant">
              ${hasFilter ? 'No signals match filter' : 'No signals found'}
            </div>`
          : this.viewMode === 'json'
            ? this.renderJsonView(filteredSignals)
            : this.renderTableView(filteredSignals)}
      </div>
    `
  }

  private renderJsonView(signals: SignalObject) {
    return html`
      <pre class="whitespace-pre-wrap break-words text-xs leading-6" .innerHTML=${renderJsonValue(signals, this.changedPaths)}></pre>
    `
  }

  private renderTableView(signals: SignalObject) {
    return html`
      <div class="overflow-x-auto">
        <table class="table table-zebra table-xs">
          <thead>
            <tr>
              <th>Signal</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody>
            ${flattenSignals(signals).map(([path, value]) => {
              const isChanged = this.changedPaths.has(path)
              return html`
                <tr class="${isChanged ? 'bg-warning' : ''}">
                  <td class="font-mono">${path}</td>
                  <td class="max-w-40 truncate font-mono" title=${JSON.stringify(value)}>
                    ${JSON.stringify(value)}
                  </td>
                </tr>
              `
            })}
          </tbody>
        </table>
      </div>
    `
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'datastar-inspector': DatastarInspector
  }
}
