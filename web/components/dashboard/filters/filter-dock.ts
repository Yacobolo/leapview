import { LitElement, css, html } from 'lit'
import { property, state } from 'lit/decorators.js'
import { SlidersHorizontal } from 'lucide'
import type {
  DashboardFilterContract,
  DashboardFilterOptionPage,
  DashboardFilterState,
  DashboardStatus,
} from '../../../generated/signals'
import { lucideIcon } from '../../shared/lucide-icons'
import './filter-control'

class LeapViewFilterDock extends LitElement {
  @property({ attribute: false }) contract?: DashboardFilterContract
  @property({ attribute: false }) filterState?: DashboardFilterState
  @property({ attribute: false }) optionPages: Record<string, DashboardFilterOptionPage> = {}
  @property({ type: String }) pageId = ''
  @property({ type: Boolean, reflect: true }) loading: DashboardStatus['loading'] = false

  @state() private open = storedFilterDockOpen()

  static styles = css`
    :host {
      display: block;
      min-width: 0;
      min-height: 0;
      color: var(--lv-fg-default);
      font-family: var(--lv-font-family-ui, var(--fontStack-system));
    }

    aside {
      display: grid;
      width: var(--lv-page-rail-width-collapsed);
      box-sizing: border-box;
      min-width: 0;
      min-height: 0;
      height: 100%;
      overflow: hidden;
      border-left: var(--lv-border-default);
      background: var(--lv-bg-panel-muted);
      transition:
        width var(--lv-duration-fast) var(--motion-easing-move),
        background-color var(--lv-duration-fast) var(--motion-easing-move);
    }

    aside[data-open] {
      grid-template-rows: minmax(0, 1fr);
      width: var(--lv-dashboard-filter-open-width);
      background: var(--lv-bg-app);
    }

    button {
      font: inherit;
    }

    .rail {
      display: flex;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      box-sizing: border-box;
      align-items: center;
      align-content: start;
      flex-direction: column;
      justify-items: center;
      justify-content: flex-start;
      gap: var(--base-size-8);
      border: 0;
      background: transparent;
      color: var(--lv-fg-muted);
      cursor: pointer;
      padding: var(--base-size-16) 0;
      font-size: var(--lv-font-size-caption);
      font-weight: var(--lv-font-weight-strong);
      text-transform: uppercase;
    }

    .rail:hover,
    .rail:focus-visible {
      color: var(--lv-fg-default);
      outline: 0;
    }

    aside[data-open] .rail {
      display: none;
    }

    .rail span {
      writing-mode: vertical-rl;
      line-height: var(--lv-line-height-none, 1);
    }

    .panel {
      display: none;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      padding: var(--base-size-12);
    }

    aside[data-open] .panel {
      display: block;
    }

    aside[data-open] .rail span {
      writing-mode: horizontal-tb;
      transform: none;
    }

    @media (max-width: 640px) {
      aside,
      aside[data-open] {
        width: 100%;
        border-left: 0;
        border-top: var(--lv-border-default);
      }

      .rail span,
      aside[data-open] .rail span {
        writing-mode: horizontal-tb;
        transform: none;
      }
    }
  `

  render() {
    return html`
      <aside ?data-open=${this.open} aria-label="Report filters">
        <button class="rail" type="button" title="Toggle filters" aria-expanded=${String(this.open)} @click=${this.toggle}>
          ${lucideIcon(SlidersHorizontal)}
          <span>Filters</span>
        </button>
        <div class="panel">
          ${this.contract && this.filterState ? this.renderCompiledPane() : html`
            <p role="status">Filter state is unavailable.</p>
          `}
        </div>
      </aside>
    `
  }

  private renderCompiledPane() {
    const bindings = Object.values(this.contract?.bindings ?? {})
      .filter((binding) => binding.paneVisible && (binding.scope === 'report' || binding.pageID === this.pageId))
      .sort((left, right) => left.paneOrder - right.paneOrder || left.key.localeCompare(right.key))
    return html`
      <header>
        <strong>Filters</strong>
        <button type="button" aria-label="Close filters" @click=${this.close}>Close</button>
      </header>
      ${bindings.map((binding) => {
        const definition = this.contract?.definitions[binding.filter]
        const expression = this.filterState?.draftControls[binding.key]
          ?? this.filterState?.appliedControls[binding.key]?.expression
          ?? binding.default
        return html`<lv-filter-pane-card
          .definition=${definition}
          .binding=${binding}
          .expression=${expression}
          .options=${this.optionPages[binding.key]}
          .pending=${this.loading}
          .stale=${this.loading}
        ></lv-filter-pane-card>`
      })}
    `
  }

  private toggle = (): void => {
    this.open = !this.open
    storeFilterDockOpen(this.open)
  }

  private close = (): void => {
    this.open = false
    storeFilterDockOpen(false)
  }
}

const filterDockStorageKey = 'leapview:filters-open'

function storedFilterDockOpen(): boolean {
  try {
    return localStorage.getItem(filterDockStorageKey) === 'open'
  } catch {
    return false
  }
}

function storeFilterDockOpen(open: boolean): void {
  try {
    localStorage.setItem(filterDockStorageKey, open ? 'open' : 'closed')
  } catch {
    // The in-memory state is enough when storage is unavailable.
  }
}

if (!customElements.get('lv-filter-dock')) customElements.define('lv-filter-dock', LeapViewFilterDock)
