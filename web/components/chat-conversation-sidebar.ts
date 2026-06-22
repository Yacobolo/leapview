import { LitElement, css, html, svg as svgTemplate, type PropertyValues } from 'lit'
import { property, state } from 'lit/decorators.js'

type Conversation = {
  id: string
  title?: string
  updatedAt?: string
}

type ChatStatus = {
  running?: boolean
}

type HoverTitle = {
  index: string
  title: string
  top: number
  active: boolean
}

const conversationsConverter = {
  fromAttribute(value: string | null): Conversation[] {
    if (!value) return []
    try {
      const parsed = JSON.parse(value)
      return Array.isArray(parsed) ? parsed as Conversation[] : []
    } catch {
      return []
    }
  },
  toAttribute(value: Conversation[]): string {
    return JSON.stringify(value ?? [])
  },
}

const statusConverter = {
  fromAttribute(value: string | null): ChatStatus {
    if (!value) return {}
    try {
      return JSON.parse(value) as ChatStatus
    } catch {
      return {}
    }
  },
  toAttribute(value: ChatStatus): string {
    return JSON.stringify(value ?? {})
  },
}

class ChatConversationSidebar extends LitElement {
  @property({ attribute: false }) conversations: Conversation[] = []
  @property({ attribute: 'conversations', converter: conversationsConverter }) conversationsAttribute: Conversation[] = []
  @property({ attribute: 'active-conversation-id' }) activeConversationId = ''
  @property({ attribute: 'status', converter: statusConverter }) status: ChatStatus = {}
  @state() private collapsed = storedCollapsed()
  @state() private hoverTitle?: HoverTitle

  static styles = css`
    :host {
      --ld-chat-conversation-sidebar-width: 176px;
      display: block;
      width: var(--ld-chat-conversation-sidebar-width);
      height: 100%;
      min-height: 0;
      overflow: hidden;
      color: var(--ld-fg-default);
      font-family: var(--fontStack-system);
      transition: width 180ms var(--ld-ease-out);
    }

    :host([data-collapsed]) {
      --ld-chat-conversation-sidebar-width: 38px;
      z-index: 30;
      overflow: visible;
    }

    aside {
      position: relative;
      display: grid;
      width: var(--ld-chat-conversation-sidebar-width);
      height: 100%;
      min-height: 0;
      grid-template-rows: auto minmax(0, 1fr);
      overflow: hidden;
      background: var(--ld-report-rail-bg);
      transition: width 180ms var(--ld-ease-out);
    }

    :host([data-collapsed]) aside {
      overflow: visible;
    }

    header {
      display: grid;
      min-width: 0;
      padding: 10px 8px;
    }

    .top-row {
      display: flex;
      min-width: 0;
      align-items: center;
      gap: 6px;
      justify-content: space-between;
    }

    .section-title {
      overflow: hidden;
      color: var(--ld-fg-default);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-strong);
      letter-spacing: 0;
      text-transform: uppercase;
    }

    .collapse {
      display: grid;
      width: 24px;
      height: 24px;
      flex: 0 0 auto;
      place-items: center;
      margin-left: auto;
      border: var(--ld-border-transparent);
      border-radius: var(--ld-radius-default);
      background: transparent;
      color: var(--ld-fg-muted);
      cursor: pointer;
      padding: 0;
    }

    .collapse:hover,
    .collapse:focus-visible {
      border-color: var(--ld-line-muted);
      background: var(--ld-bg-control-hover);
      color: var(--ld-fg-default);
      outline: 0;
    }

    .collapse svg {
      width: 14px;
      height: 14px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 2;
    }

    nav {
      display: grid;
      align-content: start;
      gap: 2px;
      min-width: 0;
      min-height: 0;
      overflow-x: hidden;
      overflow-y: auto;
      padding: 7px 5px;
      scrollbar-gutter: stable;
      scrollbar-color: var(--ld-scrollbar-thumb) transparent;
      scrollbar-width: thin;
    }

    nav::-webkit-scrollbar {
      width: 6px;
    }

    nav::-webkit-scrollbar-track {
      background: transparent;
    }

    nav::-webkit-scrollbar-thumb {
      border-radius: var(--ld-radius-full);
      background: var(--ld-scrollbar-thumb);
    }

    .conversation-button {
      position: relative;
      display: grid;
      width: 100%;
      min-height: 36px;
      grid-template-columns: 24px minmax(0, 1fr);
      align-items: center;
      gap: 6px;
      border: var(--ld-border-transparent);
      border-radius: var(--ld-radius-default);
      background: transparent;
      color: color-mix(in srgb, var(--ld-fg-muted), transparent 8%);
      cursor: pointer;
      padding: 0 9px;
      text-align: left;
    }

    .conversation-button:hover,
    .conversation-button:focus-visible {
      background: var(--ld-bg-hover);
      color: var(--ld-fg-default);
      outline: 0;
    }

    .conversation-button[aria-current='page'] {
      background: var(--ld-bg-hover);
      color: var(--ld-fg-default);
    }

    .conversation-button[aria-current='page']::before {
      content: '';
      position: absolute;
      inset-block: 7px;
      left: 0;
      width: 2px;
      border-radius: var(--ld-radius-full);
      background: var(--ld-accent);
    }

    .conversation-button:disabled {
      cursor: default;
      opacity: 0.72;
    }

    .conversation-index {
      display: grid;
      width: 24px;
      height: 24px;
      place-items: center;
      color: color-mix(in srgb, var(--ld-fg-muted), transparent 24%);
      font-size: var(--ld-font-size-caption);
      font-variant-numeric: tabular-nums;
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-none);
    }

    .conversation-button:hover .conversation-index,
    .conversation-button:focus-visible .conversation-index,
    .conversation-button[aria-current='page'] .conversation-index {
      color: var(--ld-fg-default);
    }

    .conversation-text {
      display: grid;
      min-width: 0;
      gap: 1px;
    }

    .conversation-title,
    .conversation-meta {
      overflow: hidden;
      min-width: 0;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .conversation-title {
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-medium);
      line-height: var(--ld-line-height-tight);
    }

    .conversation-button:hover .conversation-title,
    .conversation-button:focus-visible .conversation-title,
    .conversation-button[aria-current='page'] .conversation-title {
      font-weight: var(--ld-font-weight-strong);
    }

    .conversation-meta {
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      line-height: var(--ld-line-height-none);
    }

    .empty {
      padding: 8px 9px;
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      line-height: var(--ld-line-height-relaxed);
    }

    :host([data-collapsed]) header {
      padding: 8px 5px 6px;
    }

    :host([data-collapsed]) .section-title,
    :host([data-collapsed]) .conversation-text,
    :host([data-collapsed]) .empty {
      display: none;
    }

    :host([data-collapsed]) .top-row {
      display: grid;
      justify-items: center;
    }

    :host([data-collapsed]) .collapse {
      margin-left: 0;
    }

    :host([data-collapsed]) nav {
      scrollbar-gutter: auto;
      scrollbar-width: none;
    }

    :host([data-collapsed]) nav::-webkit-scrollbar {
      display: none;
      width: 0;
    }

    :host([data-collapsed]) .conversation-button {
      min-height: 29px;
      grid-template-columns: 24px;
      justify-content: center;
      padding-inline: 0;
    }

    :host([data-collapsed]) .conversation-button[aria-current='page']::before {
      inset-block: 7px;
      left: 0;
      width: 2px;
    }

    .hover-title {
      display: none;
    }

    :host([data-collapsed]) .hover-title {
      position: absolute;
      z-index: 40;
      left: 7px;
      min-height: 29px;
      max-width: 14rem;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 0 9px 0 0;
      background: var(--ld-report-rail-bg);
      color: var(--ld-fg-default);
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-strong);
      line-height: var(--ld-line-height-none);
      pointer-events: none;
      transform: translateY(-50%);
      animation: rail-title-fade-in 90ms var(--ld-ease-out);
      white-space: nowrap;
    }

    :host([data-collapsed]) .hover-title[data-active]::before {
      content: '';
      position: absolute;
      inset-block: 7px;
      left: -2px;
      width: 2px;
      border-radius: var(--ld-radius-full);
      background: var(--ld-accent);
    }

    .hover-title-index {
      display: grid;
      width: 24px;
      height: 24px;
      place-items: center;
      color: var(--ld-fg-default);
      font-variant-numeric: tabular-nums;
      font-weight: var(--ld-font-weight-strong);
    }

    .hover-title-name {
      overflow: hidden;
      text-overflow: ellipsis;
      animation: rail-title-name-fold-out 120ms var(--ld-ease-out);
      transform-origin: left center;
    }

    .rail-label {
      display: none;
    }

    :host([data-collapsed]) .rail-label {
      display: block;
      margin: 8px auto 10px;
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-strong);
      letter-spacing: 0;
      line-height: var(--ld-line-height-none);
      text-orientation: mixed;
      text-transform: uppercase;
      transform: rotate(180deg);
      writing-mode: vertical-rl;
    }

    @keyframes rail-title-fade-in {
      from {
        opacity: 0;
      }

      to {
        opacity: 1;
      }
    }

    @keyframes rail-title-name-fold-out {
      from {
        opacity: 0;
        transform: translateX(-4px) scaleX(0.86);
      }

      to {
        opacity: 1;
        transform: translateX(0) scaleX(1);
      }
    }
  `

  updated(changed: PropertyValues<this>): void {
    this.toggleAttribute('data-collapsed', this.collapsed)
    if (changed.has('conversations') || changed.has('conversationsAttribute') || changed.has('activeConversationId') || changed.has('collapsed')) {
      this.scrollActiveConversationIntoView()
    }
  }

  render() {
    const conversations = this.resolvedConversations
    return html`
      <aside aria-label="Chat conversations">
        <header>
          <div class="top-row">
            <strong class="section-title">Conversations</strong>
            <button
              class="collapse"
              type="button"
              aria-label=${this.collapsed ? 'Expand conversations' : 'Collapse conversations'}
              aria-pressed=${String(this.collapsed)}
              title=${this.collapsed ? 'Expand conversations' : 'Collapse conversations'}
              @click=${this.toggleCollapsed}
            >
              ${icon(this.collapsed ? 'chevron-right' : 'chevron-left')}
            </button>
          </div>
        </header>

        <nav aria-label="Chat conversations" @scroll=${this.hideHoverTitle}>
          <span class="rail-label" aria-hidden="true">Chats</span>
          ${conversations.length === 0 ? html`<div class="empty">No conversations yet.</div>` : null}
          ${conversations.map((conversation, index) => this.renderConversation(conversation, index, conversations.length))}
        </nav>
        ${this.collapsed && this.hoverTitle ? html`
          <div
            class="hover-title"
            style=${`top:${this.hoverTitle.top}px`}
            ?data-active=${this.hoverTitle.active}
          >
            <span class="hover-title-index" aria-hidden="true">${this.hoverTitle.index}</span>
            <span class="hover-title-name">${this.hoverTitle.title}</span>
          </div>
        ` : null}
      </aside>
    `
  }

  private get resolvedConversations(): Conversation[] {
    return Array.isArray(this.conversations) && this.conversations.length > 0 ? this.conversations : this.conversationsAttribute
  }

  private renderConversation(conversation: Conversation, index: number, count: number) {
    const active = conversation.id === this.activeConversationId
    const indexLabel = formatConversationNumber(index, count)
    const title = conversation.title || 'Conversation'
    return html`
      <button
        class="conversation-button"
        type="button"
        aria-current=${active ? 'page' : 'false'}
        aria-label=${title}
        ?disabled=${Boolean(this.status.running)}
        @click=${() => this.selectConversation(conversation.id)}
        @mouseenter=${(event: MouseEvent) => this.showHoverTitle(event, title, indexLabel, active)}
        @mouseleave=${this.hideHoverTitle}
        @focus=${(event: FocusEvent) => this.showHoverTitle(event, title, indexLabel, active)}
        @blur=${this.hideHoverTitle}
      >
        <span class="conversation-index" aria-hidden="true">${indexLabel}</span>
        <span class="conversation-text">
          <span class="conversation-title">${title}</span>
          <span class="conversation-meta">${conversation.updatedAt || ''}</span>
        </span>
      </button>
    `
  }

  private selectConversation(conversationId: string): void {
    if (!conversationId || this.status.running) return
    this.dispatchEvent(new CustomEvent('ld-chat-conversation-select', {
      bubbles: true,
      composed: true,
      detail: { conversationId },
    }))
  }

  private toggleCollapsed = () => {
    this.collapsed = !this.collapsed
    try {
      localStorage.setItem('libredash-chat-conversations-collapsed', String(this.collapsed))
    } catch {
      // Session state still updates when storage is unavailable.
    }
  }

  private scrollActiveConversationIntoView(): void {
    requestAnimationFrame(() => {
      const active = this.renderRoot.querySelector<HTMLElement>('.conversation-button[aria-current="page"]')
      active?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    })
  }

  private showHoverTitle(event: MouseEvent | FocusEvent, title: string, index: string, active: boolean): void {
    if (!this.collapsed) return
    const target = event.currentTarget
    const aside = this.renderRoot.querySelector<HTMLElement>('aside')
    if (!(target instanceof HTMLElement) || !aside) return
    const targetRect = target.getBoundingClientRect()
    const asideRect = aside.getBoundingClientRect()
    this.hoverTitle = {
      index,
      title,
      top: targetRect.top - asideRect.top + targetRect.height / 2,
      active,
    }
  }

  private hideHoverTitle = (): void => {
    this.hoverTitle = undefined
  }
}

function storedCollapsed(): boolean {
  try {
    return localStorage.getItem('libredash-chat-conversations-collapsed') === 'true'
  } catch {
    return false
  }
}

function formatConversationNumber(index: number, count: number): string {
  const number = String(index + 1)
  return count >= 10 ? number.padStart(2, '0') : number
}

function icon(name: 'chevron-left' | 'chevron-right') {
  switch (name) {
    case 'chevron-left':
      return iconSvg(svgTemplate`<path d="m15 18-6-6 6-6"></path>`)
    case 'chevron-right':
      return iconSvg(svgTemplate`<path d="m9 18 6-6-6-6"></path>`)
  }
}

function iconSvg(content: unknown) {
  return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`
}

if (!customElements.get('ld-chat-conversation-sidebar')) {
  customElements.define('ld-chat-conversation-sidebar', ChatConversationSidebar)
}
