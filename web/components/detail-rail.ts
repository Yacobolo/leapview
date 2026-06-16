const storageKey = 'libredash.metricDetailRail'

class DetailRail extends HTMLElement {
  private button?: HTMLButtonElement
  private collapsed = false

  connectedCallback(): void {
    this.collapsed = this.savedState()
    this.ensureToggle()
    this.sync()
  }

  private ensureToggle(): void {
    if (this.button) return
    const header = this.querySelector<HTMLElement>('.metric-info-header')
    if (!header) return
    this.button = document.createElement('button')
    this.button.type = 'button'
    this.button.className = 'metric-rail-toggle'
    this.button.addEventListener('click', () => this.toggle())
    header.append(this.button)
  }

  private toggle(): void {
    this.collapsed = !this.collapsed
    try {
      window.localStorage.setItem(storageKey, this.collapsed ? 'collapsed' : 'expanded')
    } catch {
      // The rail still works if storage is unavailable.
    }
    this.sync()
  }

  private sync(): void {
    this.toggleAttribute('data-rail-collapsed', this.collapsed)
    if (this.collapsed) {
      document.documentElement.setAttribute('data-metric-detail-rail', 'collapsed')
    } else {
      document.documentElement.removeAttribute('data-metric-detail-rail')
    }
    if (!this.button) return
    this.button.setAttribute('aria-expanded', String(!this.collapsed))
    this.button.setAttribute('aria-label', this.collapsed ? 'Expand details' : 'Collapse details')
    this.button.title = this.collapsed ? 'Expand details' : 'Collapse details'
    this.button.textContent = this.collapsed ? '<' : '>'
  }

  private savedState(): boolean {
    try {
      return window.localStorage.getItem(storageKey) === 'collapsed'
    } catch {
      return false
    }
  }
}

customElements.define('ld-detail-rail', DetailRail)
