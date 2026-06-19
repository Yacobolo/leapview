import { LitElement, css, html, nothing } from 'lit'
import { property, state } from 'lit/decorators.js'

type Workspace = {
  id?: string
  title?: string
}

type Role = {
  name: string
}

type Binding = {
  principalId: string
  email: string
  displayName?: string
  role: string
}

type AccessStatus = {
  loading?: boolean
  error?: string
  message?: string
}

type WorkspaceAccess = {
  workspace?: Workspace
  roles?: Role[]
  bindings?: Binding[]
  canManage?: boolean
  status?: AccessStatus
}

type AccessCommand = {
  email?: string
  displayName?: string
  role?: string
  principalId?: string
}

const emptyAccess: WorkspaceAccess = {
  roles: [],
  bindings: [],
  canManage: false,
  status: {},
}

const focusableSelector = [
  'a[href]:not([tabindex="-1"])',
  'button:not([disabled]):not([tabindex="-1"])',
  'input:not([disabled]):not([tabindex="-1"])',
  'select:not([disabled]):not([tabindex="-1"])',
  'textarea:not([disabled]):not([tabindex="-1"])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

class WorkspaceAccessControl extends LitElement {
  @property({ attribute: false }) access: WorkspaceAccess | null = null
  @property({ attribute: 'access' }) accessAttribute = ''

  @state() private open = false
  @state() private email = ''
  @state() private displayName = ''
  @state() private selectedRole = 'viewer'
  @state() private query = ''

  private previousFocus: HTMLElement | null = null

  static styles = css`
    :host {
      display: inline-block;
      color: var(--ld-fg-default);
      font-family: var(--ld-font-family-ui);
    }

    button,
    input,
    select {
      font: inherit;
    }

    .trigger {
      display: inline-flex;
      min-height: var(--ld-control-medium);
      align-items: center;
      justify-content: center;
      gap: var(--ld-space-sm);
      border: var(--ld-border-transparent);
      border-radius: var(--ld-radius-default);
      background: transparent;
      color: var(--ld-fg-muted);
      cursor: pointer;
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-medium);
      line-height: var(--ld-line-height-tight);
      padding: 0 var(--ld-space-lg);
      transition:
        color var(--ld-transition-fast),
        background-color var(--ld-transition-fast),
        border-color var(--ld-transition-fast);
    }

    .trigger:hover,
    .trigger:focus-visible {
      border-color: var(--ld-line-muted);
      background: var(--ld-bg-control-hover);
      color: var(--ld-fg-default);
      outline: 0;
    }

    .icon {
      display: inline-flex;
      width: var(--ld-icon-sm);
      height: var(--ld-icon-sm);
      align-items: center;
      justify-content: center;
      color: currentColor;
    }

    .overlay {
      position: fixed;
      inset: 0;
      z-index: calc(var(--z-index-inspector) - 1);
      display: grid;
      place-items: start center;
      background: var(--ld-modal-backdrop);
      padding: var(--base-size-32) var(--base-size-16);
    }

    .dialog {
      display: grid;
      width: min(34rem, calc(100vw - var(--base-size-32)));
      max-height: calc(100vh - var(--base-size-64));
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-large);
      background: var(--ld-bg-panel);
      box-shadow: var(--ld-shadow-floating-lg);
    }

    .header,
    .footer {
      border-bottom: var(--ld-border-muted);
      padding: var(--base-size-16);
    }

    .header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: var(--base-size-16);
    }

    .title {
      margin: 0;
      color: var(--ld-fg-default);
      font-size: var(--ld-font-size-title-sm);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-snug);
    }

    .subtitle {
      margin: var(--base-size-4) 0 0;
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-normal);
      line-height: var(--ld-line-height-snug);
    }

    .close,
    .row-action {
      display: inline-flex;
      width: var(--ld-control-medium);
      height: var(--ld-control-medium);
      flex: 0 0 auto;
      align-items: center;
      justify-content: center;
      border: var(--ld-border-transparent);
      border-radius: var(--ld-radius-default);
      background: transparent;
      color: var(--ld-fg-muted);
      cursor: pointer;
      padding: 0;
      transition:
        color var(--ld-transition-fast),
        background-color var(--ld-transition-fast),
        border-color var(--ld-transition-fast);
    }

    .close:hover,
    .close:focus-visible,
    .row-action:hover,
    .row-action:focus-visible {
      border-color: var(--ld-line-muted);
      background: var(--ld-bg-control-hover);
      color: var(--ld-fg-default);
      outline: 0;
    }

    .body {
      min-height: 0;
      overflow: auto;
      padding: var(--base-size-16);
    }

    .section {
      display: grid;
      gap: var(--base-size-12);
    }

    .section + .section {
      margin-top: var(--base-size-20);
      border-top: var(--ld-border-muted);
      padding-top: var(--base-size-16);
    }

    .section-title {
      margin: 0;
      color: var(--ld-fg-default);
      font-size: var(--ld-font-size-body-md);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-snug);
    }

    .form {
      display: grid;
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(8rem, auto) auto;
      gap: var(--base-size-8);
    }

    label {
      display: grid;
      min-width: 0;
      gap: var(--base-size-4);
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-medium);
      line-height: var(--ld-line-height-tight);
      text-transform: uppercase;
    }

    input,
    select {
      min-height: var(--ld-control-medium);
      min-width: 0;
      border: var(--ld-border-default);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      color: var(--ld-fg-default);
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-medium);
      line-height: var(--ld-line-height-snug);
      padding: 0 var(--base-size-8);
    }

    input::placeholder {
      color: var(--ld-fg-muted);
    }

    input:focus,
    select:focus {
      border-color: var(--ld-line-accent);
      outline: 2px solid var(--ld-line-accent-muted);
      outline-offset: 0;
    }

    .submit {
      align-self: end;
      min-height: var(--ld-control-medium);
      border: 0;
      border-radius: var(--ld-radius-default);
      background: var(--button-primary-bgColor-rest);
      color: var(--button-primary-fgColor-rest);
      cursor: pointer;
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-tight);
      padding: 0 var(--base-size-12);
    }

    .submit:hover,
    .submit:focus-visible {
      background: var(--button-primary-bgColor-hover);
      outline: 0;
    }

    .submit:disabled,
    .row-action:disabled {
      cursor: not-allowed;
      opacity: var(--opacity-disabled);
    }

    .status {
      border-radius: var(--ld-radius-default);
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-medium);
      line-height: var(--ld-line-height-snug);
      padding: var(--base-size-8) var(--base-size-12);
    }

    .status-error {
      border: var(--ld-border-danger);
      background: var(--ld-bg-danger-muted);
      color: var(--ld-fg-danger);
    }

    .status-message {
      border: 1px solid var(--ld-line-success-muted);
      background: var(--ld-bg-success-muted);
      color: var(--ld-fg-success);
    }

    .toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--base-size-12);
    }

    .search {
      width: min(18rem, 100%);
    }

    .list {
      display: grid;
      border-top: var(--ld-border-muted);
    }

    .row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) minmax(8rem, 10rem) auto;
      align-items: center;
      gap: var(--base-size-12);
      border-bottom: var(--ld-border-muted);
      padding: var(--base-size-10) 0;
    }

    .person {
      min-width: 0;
    }

    .name {
      overflow: hidden;
      margin: 0;
      color: var(--ld-fg-default);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-snug);
    }

    .email {
      overflow: hidden;
      margin: var(--base-size-2) 0 0;
      color: var(--ld-fg-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-normal);
      line-height: var(--ld-line-height-tight);
    }

    .empty {
      border: 1px dashed var(--ld-line-muted);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-panel-muted);
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-medium);
      padding: var(--base-size-16);
    }

    @media (max-width: 44rem) {
      .overlay {
        align-items: end;
        padding: var(--base-size-8);
      }

      .dialog {
        width: 100%;
        max-height: calc(100vh - var(--base-size-16));
      }

      .form,
      .row {
        grid-template-columns: minmax(0, 1fr);
      }

      .submit {
        justify-self: start;
      }
    }
  `

  updated(changed: Map<string, unknown>): void {
    if (changed.has('access') || changed.has('accessAttribute')) {
      this.ensureRole()
      const status = this.resolvedAccess.status
      if (status?.message && !status.error && !status.loading) {
        this.email = ''
        this.displayName = ''
      }
    }
  }

  render() {
    const access = this.resolvedAccess
    if (!access.canManage) return nothing

    return html`
      <button class="trigger" type="button" aria-haspopup="dialog" aria-expanded=${String(this.open)} @click=${this.openDialog}>
        ${shieldIcon()}
        <span>Access</span>
      </button>
      ${this.open ? this.renderModal(access) : nothing}
    `
  }

  private renderModal(access: WorkspaceAccess) {
    const status = access.status ?? {}
    return html`
      <div class="overlay" @click=${this.handleOverlayClick}>
        <section
          class="dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="workspace-access-title"
          @keydown=${this.handleKeyDown}
        >
          <header class="header">
            <div>
              <h2 class="title" id="workspace-access-title">Manage access</h2>
              <p class="subtitle">${access.workspace?.title ?? 'Workspace'} roles apply to every published asset in this workspace.</p>
            </div>
            <button class="close" type="button" aria-label="Close workspace access" @click=${this.closeDialog}>
              ${xIcon()}
            </button>
          </header>
          <div class="body">
            <section class="section" aria-label="Add workspace access">
              <h3 class="section-title">Assign role</h3>
              ${status.error ? html`<div class="status status-error" role="alert">${status.error}</div>` : nothing}
              ${status.message && !status.error ? html`<div class="status status-message" role="status">${status.message}</div>` : nothing}
              <form class="form" @submit=${this.handleSubmit}>
                <label>
                  Email
                  <input
                    type="email"
                    autocomplete="email"
                    placeholder="person@example.com"
                    .value=${this.email}
                    ?disabled=${status.loading}
                    @input=${(event: Event) => { this.email = (event.currentTarget as HTMLInputElement).value }}
                  >
                </label>
                <label>
                  Display name
                  <input
                    type="text"
                    autocomplete="name"
                    placeholder="Optional"
                    .value=${this.displayName}
                    ?disabled=${status.loading}
                    @input=${(event: Event) => { this.displayName = (event.currentTarget as HTMLInputElement).value }}
                  >
                </label>
                <label>
                  Role
                  <select
                    .value=${this.selectedRole}
                    ?disabled=${status.loading}
                    @change=${(event: Event) => { this.selectedRole = (event.currentTarget as HTMLSelectElement).value }}
                  >
                    ${this.roles.map((role) => html`<option value=${role.name}>${roleLabel(role.name)}</option>`)}
                  </select>
                </label>
                <button class="submit" type="submit" ?disabled=${status.loading || !this.email.trim() || !this.selectedRole}>
                  ${status.loading ? 'Saving' : 'Assign'}
                </button>
              </form>
            </section>
            <section class="section" aria-label="Current workspace access">
              <div class="toolbar">
                <h3 class="section-title">Current access</h3>
                <input
                  class="search"
                  type="search"
                  placeholder="Search access..."
                  .value=${this.query}
                  @input=${(event: Event) => { this.query = (event.currentTarget as HTMLInputElement).value }}
                >
              </div>
              ${this.renderBindings(access)}
            </section>
          </div>
        </section>
      </div>
    `
  }

  private renderBindings(access: WorkspaceAccess) {
    const rows = this.filteredBindings(access.bindings ?? [])
    if (rows.length === 0) {
      return html`<div class="empty">${this.query ? 'No access entries match this search.' : 'No role bindings yet.'}</div>`
    }
    return html`
      <div class="list">
        ${rows.map((binding) => html`
          <div class="row">
            <div class="person">
              <p class="name">${displayLabel(binding)}</p>
              <p class="email">${binding.email}</p>
            </div>
            <select
              aria-label=${`Role for ${displayLabel(binding)}`}
              .value=${binding.role}
              ?disabled=${access.status?.loading}
              @change=${(event: Event) => this.updateBindingRole(binding, (event.currentTarget as HTMLSelectElement).value)}
            >
              ${this.roles.map((role) => html`<option value=${role.name}>${roleLabel(role.name)}</option>`)}
            </select>
            <button
              class="row-action"
              type="button"
              aria-label=${`Remove ${displayLabel(binding)}`}
              ?disabled=${access.status?.loading}
              @click=${() => this.removeBinding(binding)}
            >
              ${trashIcon()}
            </button>
          </div>
        `)}
      </div>
    `
  }

  private get resolvedAccess(): WorkspaceAccess {
    if (this.access) return normalizeAccess(this.access)
    if (this.accessAttribute) {
      try {
        return normalizeAccess(JSON.parse(this.accessAttribute) as WorkspaceAccess)
      } catch {
        return emptyAccess
      }
    }
    return emptyAccess
  }

  private get roles(): Role[] {
    return this.resolvedAccess.roles ?? []
  }

  private ensureRole(): void {
    const roles = this.roles
    if (roles.some((role) => role.name === this.selectedRole)) return
    this.selectedRole = roles.find((role) => role.name === 'viewer')?.name ?? roles[0]?.name ?? ''
  }

  private filteredBindings(bindings: Binding[]): Binding[] {
    const query = this.query.trim().toLowerCase()
    if (!query) return bindings
    return bindings.filter((binding) => {
      return `${binding.displayName ?? ''} ${binding.email} ${binding.role}`.toLowerCase().includes(query)
    })
  }

  private readonly openDialog = (): void => {
    this.previousFocus = document.activeElement as HTMLElement | null
    this.open = true
    window.setTimeout(() => {
      const first = this.focusableElements()[0]
      first?.focus()
    }, 0)
  }

  private readonly closeDialog = (): void => {
    this.open = false
    window.setTimeout(() => {
      if (this.previousFocus?.isConnected) this.previousFocus.focus()
      this.previousFocus = null
    }, 0)
  }

  private readonly handleOverlayClick = (event: Event): void => {
    if (event.target === event.currentTarget) this.closeDialog()
  }

  private readonly handleKeyDown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape') {
      event.preventDefault()
      this.closeDialog()
      return
    }
    if (event.key !== 'Tab') return
    const focusable = this.focusableElements()
    if (focusable.length === 0) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && this.shadowRoot?.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && this.shadowRoot?.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  private focusableElements(): HTMLElement[] {
    const dialog = this.renderRoot.querySelector<HTMLElement>('.dialog')
    if (!dialog) return []
    return Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector))
  }

  private readonly handleSubmit = (event: Event): void => {
    event.preventDefault()
    const command: AccessCommand = {
      email: this.email.trim(),
      displayName: this.displayName.trim(),
      role: this.selectedRole,
    }
    if (!command.email || !command.role) return
    this.dispatchEvent(new CustomEvent('ld-workspace-access-upsert', {
      bubbles: true,
      composed: true,
      detail: command,
    }))
  }

  private updateBindingRole(binding: Binding, role: string): void {
    if (!binding.email || !role || role === binding.role) return
    this.dispatchEvent(new CustomEvent('ld-workspace-access-upsert', {
      bubbles: true,
      composed: true,
      detail: {
        email: binding.email,
        displayName: binding.displayName,
        role,
      },
    }))
  }

  private removeBinding(binding: Binding): void {
    if (!binding.principalId) return
    this.dispatchEvent(new CustomEvent('ld-workspace-access-remove', {
      bubbles: true,
      composed: true,
      detail: {
        principalId: binding.principalId,
      },
    }))
  }
}

function normalizeAccess(access: WorkspaceAccess): WorkspaceAccess {
  return {
    workspace: access.workspace ?? {},
    roles: access.roles ?? [],
    bindings: access.bindings ?? [],
    canManage: Boolean(access.canManage),
    status: access.status ?? {},
  }
}

function displayLabel(binding: Binding): string {
  return binding.displayName?.trim() || binding.email || 'Principal'
}

function roleLabel(role: string): string {
  return role.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

function shieldIcon() {
  return html`<span class="icon" aria-hidden="true"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13c0 5-3.5 7.5-7.7 8.9a1 1 0 0 1-.6 0C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.2-2.7a1.2 1.2 0 0 1 1.6 0C14.5 3.8 17 5 19 5a1 1 0 0 1 1 1z"></path></svg></span>`
}

function xIcon() {
  return html`<span class="icon" aria-hidden="true"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg></span>`
}

function trashIcon() {
  return html`<span class="icon" aria-hidden="true"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 11v6"></path><path d="M14 11v6"></path><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"></path><path d="M3 6h18"></path><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg></span>`
}

customElements.define('ld-workspace-access-control', WorkspaceAccessControl)

declare global {
  interface HTMLElementTagNameMap {
    'ld-workspace-access-control': WorkspaceAccessControl
  }
}
