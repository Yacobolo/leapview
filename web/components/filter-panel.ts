import { LitElement, css, html, nothing } from 'lit'
import { property, state } from 'lit/decorators.js'

type FilterType = 'date_range' | 'multi_select' | 'text'

type FilterDefinition = {
  type: FilterType
  label: string
  dataset: string
  dimension: string
  default?: FilterDefault
  custom?: boolean
  presets?: DatePreset[]
  operator?: string
  values?: { source?: string; limit?: number }
  defaultOperator?: string
  operators?: string[]
}

type FilterDefault = {
  preset?: string
  from?: string
  to?: string
  operator?: string
  value?: string
  values?: string[]
}

type DatePreset = {
  value: string
  label: string
  from?: string
  to?: string
  relativeDays?: number
}

type FilterControl = {
  type: FilterType | string
  operator?: string
  preset?: string
  from?: string
  to?: string
  value?: string
  values?: string[]
}

type VisualSelection = {
  label?: string
  values?: string[]
}

type FiltersSignal = {
  controls: Record<string, FilterControl>
  visualSelections: VisualSelection[]
}

type FilterOption = {
  value: string
  label: string
}

const emptyFilters: FiltersSignal = { controls: {}, visualSelections: [] }

const jsonConverter = <T,>(fallback: T) => ({
  fromAttribute(value: string | null): T {
    if (!value) return fallback
    try {
      return JSON.parse(value) as T
    } catch {
      return fallback
    }
  },
  toAttribute(value: T | null): string {
    return JSON.stringify(value ?? fallback)
  },
})

class FilterPanel extends LitElement {
  @property({ attribute: 'config', converter: jsonConverter<Record<string, FilterDefinition>>({}) }) config: Record<string, FilterDefinition> = {}
  @property({ attribute: 'filters', converter: jsonConverter<FiltersSignal>(emptyFilters) }) filters: FiltersSignal = emptyFilters
  @property({ attribute: 'options', converter: jsonConverter<Record<string, FilterOption[]>>({}) }) options: Record<string, FilterOption[]> = {}
  @property({ type: Boolean, reflect: true }) loading = false
  @state() private searches: Record<string, string> = {}

  static styles = css`
    :host {
      display: block;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    .panel {
      display: grid;
      gap: 8px;
      font-size: 11px;
    }

    header,
    .summary {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }

    header {
      border-bottom: 1px solid var(--borderColor-default);
      padding-bottom: 7px;
    }

    h2 {
      margin: 0;
      font-size: 0.78rem;
      font-weight: 850;
      line-height: 1.15;
    }

    .count {
      border: 1px solid var(--borderColor-default);
      border-radius: 999px;
      background: var(--bgColor-muted);
      color: var(--fgColor-muted);
      padding: 2px 6px;
      font-size: 0.58rem;
      font-weight: 900;
      line-height: 1;
      white-space: nowrap;
    }

    .card {
      display: grid;
      gap: 6px;
      border: 1px solid var(--borderColor-muted);
      border-radius: 5px;
      background: color-mix(in srgb, var(--report-panel, var(--bgColor-default)), transparent 18%);
      padding: 8px;
    }

    .card-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }

    h3 {
      margin: 0;
      color: var(--fgColor-muted);
      font-size: 0.58rem;
      font-weight: 900;
      text-transform: uppercase;
    }

    button,
    input,
    select {
      font: inherit;
    }

    .clear,
    .reset {
      border: 1px solid var(--borderColor-default);
      border-radius: 4px;
      background: var(--bgColor-default);
      color: var(--fgColor-muted);
      cursor: pointer;
      padding: 3px 6px;
      font-size: 0.6rem;
      font-weight: 850;
    }

    .clear:disabled,
    .reset:disabled,
    .refresh:disabled {
      cursor: default;
      opacity: 0.55;
    }

    .input-row {
      display: grid;
      grid-template-columns: 1fr;
      gap: 6px;
    }

    .date-row {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 6px;
    }

    input,
    select {
      width: 100%;
      min-width: 0;
      min-height: 25px;
      border: 1px solid var(--borderColor-default);
      border-radius: 4px;
      background: var(--control-bgColor-rest);
      color: var(--fgColor-default);
      padding: 0 7px;
      font-size: 0.7rem;
      font-weight: 650;
      outline-offset: 2px;
    }

    input:focus,
    select:focus {
      outline: 2px solid var(--ld-accent);
    }

    .checks {
      display: grid;
      max-height: 138px;
      gap: 2px;
      overflow: auto;
    }

    label.check {
      display: flex;
      min-width: 0;
      align-items: center;
      gap: 6px;
      border-radius: 4px;
      padding: 3px 4px;
      color: var(--fgColor-default);
      font-size: 0.68rem;
      font-weight: 700;
    }

    label.check:hover {
      background: var(--bgColor-muted);
    }

    label.check input {
      width: 13px;
      height: 13px;
      min-height: 0;
      accent-color: var(--ld-accent);
    }

    .empty {
      color: var(--fgColor-muted);
      font-size: 0.65rem;
      font-weight: 750;
      padding: 4px;
    }

    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    .chip {
      max-width: 100%;
      overflow: hidden;
      border: 1px solid var(--borderColor-muted);
      border-radius: 999px;
      background: var(--bgColor-muted);
      color: var(--fgColor-muted);
      padding: 2px 6px;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.58rem;
      font-weight: 850;
    }

    .summary {
      min-height: 24px;
      color: var(--fgColor-muted);
      font-size: 0.63rem;
      font-weight: 800;
    }

    .refresh {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 5px;
      min-height: 27px;
      width: 100%;
      cursor: pointer;
      border: 1px solid var(--button-primary-bgColor-rest);
      border-radius: 4px;
      background: var(--button-primary-bgColor-rest);
      color: var(--button-primary-fgColor-rest);
      font-size: 0.7rem;
      font-weight: 850;
    }
  `

  render() {
    const names = Object.keys(this.config).sort()
    const activeCount = this.activeCount()
    return html`
      <section class="panel" aria-label="Filters">
        <header>
          <h2>Filters</h2>
          <span class="count">${activeCount} active</span>
        </header>
        ${names.map((name) => this.renderFilter(name, this.config[name]))}
        ${this.renderVisualSelections()}
        <div class="summary">
          <span>${activeCount} total filter${activeCount === 1 ? '' : 's'} applied</span>
          <button class="reset" type="button" ?disabled=${this.loading || activeCount === 0} @click=${this.reset}>Reset</button>
        </div>
        <button class="refresh" type="button" ?disabled=${this.loading} @click=${this.refresh}>Refresh</button>
      </section>
    `
  }

  private renderFilter(name: string, definition: FilterDefinition) {
    const control = this.control(name, definition)
    return html`
      <article class="card">
        <div class="card-head">
          <h3>${definition.label}</h3>
          <button class="clear" type="button" ?disabled=${!this.isActive(name, definition)} @click=${() => this.clearFilter(name)}>Clear</button>
        </div>
        ${definition.type === 'date_range' ? this.renderDate(name, definition, control) : nothing}
        ${definition.type === 'multi_select' ? this.renderMulti(name, definition, control) : nothing}
        ${definition.type === 'text' ? this.renderText(name, definition, control) : nothing}
      </article>
    `
  }

  private renderDate(name: string, definition: FilterDefinition, control: FilterControl) {
    const preset = control.preset || definition.default?.preset || 'all'
    const showCustom = definition.custom && (preset === 'custom' || control.from || control.to)
    return html`
      <div class="input-row">
        <select aria-label=${definition.label} .value=${showCustom ? 'custom' : preset} @change=${(event: Event) => this.setDatePreset(name, event)}>
          ${(definition.presets ?? []).map((item) => html`<option value=${item.value}>${item.label}</option>`)}
          ${definition.custom ? html`<option value="custom">Custom range</option>` : nothing}
        </select>
        ${showCustom
          ? html`<div class="date-row">
              <input type="date" aria-label="${definition.label} from" .value=${control.from ?? ''} @input=${(event: Event) => this.setDateValue(name, 'from', event)} />
              <input type="date" aria-label="${definition.label} to" .value=${control.to ?? ''} @input=${(event: Event) => this.setDateValue(name, 'to', event)} />
            </div>`
          : nothing}
      </div>
    `
  }

  private renderMulti(name: string, definition: FilterDefinition, control: FilterControl) {
    const search = this.searches[name]?.toLowerCase() ?? ''
    const selected = new Set(control.values ?? [])
    const options = (this.options[name] ?? []).filter((option) => option.label.toLowerCase().includes(search) || option.value.toLowerCase().includes(search))
    return html`
      <div class="input-row">
        <input type="search" placeholder="Search ${definition.label.toLowerCase()}..." .value=${this.searches[name] ?? ''} @input=${(event: Event) => this.setSearch(name, event)} />
        <div class="checks">
          ${options.length === 0 ? html`<div class="empty">No values loaded</div>` : nothing}
          ${options.map((option) => html`
            <label class="check">
              <input type="checkbox" .checked=${selected.has(option.value)} @change=${() => this.toggleValue(name, option.value)} />
              <span>${option.label}</span>
            </label>
          `)}
        </div>
      </div>
    `
  }

  private renderText(name: string, definition: FilterDefinition, control: FilterControl) {
    return html`
      <div class="input-row">
        <select aria-label="${definition.label} operator" .value=${control.operator ?? definition.defaultOperator ?? 'contains'} @change=${(event: Event) => this.setOperator(name, event)}>
          ${(definition.operators ?? ['contains']).map((operator) => html`<option value=${operator}>${operatorLabel(operator)}</option>`)}
        </select>
        <input type="search" placeholder="health, watches, furniture..." .value=${control.value ?? ''} @input=${(event: Event) => this.setTextValue(name, event)} />
      </div>
    `
  }

  private renderVisualSelections() {
    const selections = this.filters.visualSelections ?? []
    if (selections.length === 0) return nothing
    return html`
      <article class="card">
        <div class="card-head">
          <h3>Visual selections</h3>
          <button class="clear" type="button" @click=${this.clearVisualSelections}>Clear</button>
        </div>
        <div class="chips">
          ${selections.map((selection) => html`<span class="chip">${selection.label || (selection.values ?? []).join(', ')}</span>`)}
        </div>
      </article>
    `
  }

  private control(name: string, definition: FilterDefinition): FilterControl {
    return this.filters.controls?.[name] ?? defaultControl(definition)
  }

  private nextFilters(): FiltersSignal {
    return {
      controls: { ...(this.filters.controls ?? {}) },
      visualSelections: [...(this.filters.visualSelections ?? [])],
    }
  }

  private emitChange(filters: FiltersSignal): void {
    this.dispatchEvent(new CustomEvent('ld-filters-change', { detail: { filters }, bubbles: true, composed: true }))
  }

  private updateControl(name: string, control: FilterControl): void {
    const filters = this.nextFilters()
    filters.controls[name] = control
    this.emitChange(filters)
  }

  private setDatePreset(name: string, event: Event): void {
    const value = (event.currentTarget as HTMLSelectElement).value
    const definition = this.config[name]
    const control = this.control(name, definition)
    this.updateControl(name, {
      ...control,
      type: 'date_range',
      preset: value,
      from: value === 'custom' ? control.from ?? '' : '',
      to: value === 'custom' ? control.to ?? '' : '',
    })
  }

  private setDateValue(name: string, key: 'from' | 'to', event: Event): void {
    const definition = this.config[name]
    const control = this.control(name, definition)
    this.updateControl(name, { ...control, type: 'date_range', preset: 'custom', [key]: (event.currentTarget as HTMLInputElement).value })
  }

  private toggleValue(name: string, value: string): void {
    const definition = this.config[name]
    const control = this.control(name, definition)
    const selected = new Set(control.values ?? [])
    if (selected.has(value)) {
      selected.delete(value)
    } else {
      selected.add(value)
    }
    this.updateControl(name, { ...control, type: 'multi_select', operator: 'in', values: [...selected].sort() })
  }

  private setOperator(name: string, event: Event): void {
    const definition = this.config[name]
    const control = this.control(name, definition)
    this.updateControl(name, { ...control, type: 'text', operator: (event.currentTarget as HTMLSelectElement).value })
  }

  private setTextValue(name: string, event: Event): void {
    const definition = this.config[name]
    const control = this.control(name, definition)
    this.updateControl(name, { ...control, type: 'text', value: (event.currentTarget as HTMLInputElement).value })
  }

  private setSearch(name: string, event: Event): void {
    this.searches = { ...this.searches, [name]: (event.currentTarget as HTMLInputElement).value }
  }

  private clearFilter(name: string): void {
    const definition = this.config[name]
    this.updateControl(name, defaultControl(definition))
  }

  private clearVisualSelections = (): void => {
    this.dispatchEvent(new CustomEvent('ld-visual-selection-clear', { bubbles: true, composed: true }))
  }

  private reset = (): void => {
    const filters: FiltersSignal = { controls: {}, visualSelections: [] }
    for (const [name, definition] of Object.entries(this.config)) {
      filters.controls[name] = defaultControl(definition)
    }
    this.dispatchEvent(new CustomEvent('ld-filters-reset', { detail: { filters }, bubbles: true, composed: true }))
  }

  private refresh = (): void => {
    this.dispatchEvent(new CustomEvent('ld-filters-refresh', { bubbles: true, composed: true }))
  }

  private activeCount(): number {
    let count = this.filters.visualSelections?.length ?? 0
    for (const [name, definition] of Object.entries(this.config)) {
      if (this.isActive(name, definition)) count += 1
    }
    return count
  }

  private isActive(name: string, definition: FilterDefinition): boolean {
    const control = this.control(name, definition)
    switch (definition.type) {
      case 'date_range':
        return Boolean(control.from || control.to || ((control.preset || definition.default?.preset || 'all') !== (definition.default?.preset || 'all')))
      case 'multi_select':
        return (control.values ?? []).length > 0
      case 'text':
        return Boolean((control.value ?? '').trim())
      default:
        return false
    }
  }
}

function defaultControl(definition: FilterDefinition): FilterControl {
  switch (definition.type) {
    case 'date_range':
      return { type: 'date_range', preset: definition.default?.preset || 'all', from: definition.default?.from || '', to: definition.default?.to || '' }
    case 'multi_select':
      return { type: 'multi_select', operator: definition.operator || 'in', values: [...(definition.default?.values ?? [])] }
    case 'text':
      return { type: 'text', operator: definition.default?.operator || definition.defaultOperator || 'contains', value: definition.default?.value || '' }
    default:
      return { type: definition.type || '' }
  }
}

function operatorLabel(operator: string): string {
  switch (operator) {
    case 'equals':
      return 'Equals'
    case 'starts_with':
      return 'Starts with'
    case 'not_contains':
      return 'Does not contain'
    default:
      return 'Contains'
  }
}

customElements.define('ld-filter-panel', FilterPanel)
