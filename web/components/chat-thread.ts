import { LitElement, css, html, nothing } from 'lit'
import { property } from 'lit/decorators.js'
import { unsafeHTML } from 'lit/directives/unsafe-html.js'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

type ChatStatus = {
  enabled?: boolean
  running?: boolean
  error?: string
}

type ChatTranscriptItem = {
  id: string
  kind: 'user' | 'assistant' | 'tool' | 'error' | 'summary' | string
  text?: string
  markdown?: string
  toolCallId?: string
  name?: string
  title?: string
  status?: 'running' | 'complete' | 'error' | 'streaming' | string
  summary?: string
  error?: string
  conversationId?: string
  runId?: string
  createdAt?: string
}

const markdown = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false,
})

const jsonConverter = <T,>(fallback: T) => ({
  fromAttribute(value: string | null): T {
    if (!value) return fallback
    try {
      return JSON.parse(value) as T
    } catch {
      return fallback
    }
  },
  toAttribute(value: T): string {
    return JSON.stringify(value ?? fallback)
  },
})

class ChatThread extends LitElement {
  @property({ attribute: false }) transcript: ChatTranscriptItem[] = []
  @property({ attribute: 'transcript', converter: jsonConverter<ChatTranscriptItem[]>([]) }) transcriptAttribute: ChatTranscriptItem[] = []
  @property({ attribute: 'status', converter: jsonConverter<ChatStatus>({}) }) status: ChatStatus = {}
  @property({ attribute: 'conversation-id' }) conversationId = ''

  static styles = css`
    :host {
      box-sizing: border-box;
      display: block;
      height: 100%;
      min-height: 0;
      overflow: hidden;
      color: var(--ld-fg-default);
      font-family: var(--fontStack-system);
    }

    *,
    *::before,
    *::after {
      box-sizing: inherit;
    }

    .thread {
      display: grid;
      height: 100%;
      min-height: 0;
      grid-template-rows: minmax(0, 1fr);
      overflow: hidden;
      background: var(--ld-bg-page);
    }

    .scroll {
      height: 100%;
      min-height: 0;
      overflow: auto;
      overscroll-behavior: contain;
      padding: var(--ld-chat-thread-padding);
    }

    .stack {
      display: grid;
      width: min(100%, var(--ld-chat-stack-width));
      margin-inline: auto;
      gap: var(--ld-chat-stack-gap);
    }

    .empty,
    .alert {
      display: grid;
      gap: var(--ld-space-sm);
      align-content: center;
      min-height: var(--ld-chat-empty-min-height);
      border: var(--ld-border-muted);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-panel);
      padding: var(--ld-chat-thread-padding);
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-body-sm);
      text-align: center;
    }

    .alert {
      min-height: auto;
      border-color: var(--ld-line-danger-muted);
      background: var(--ld-bg-danger-muted);
      color: var(--ld-fg-default);
      text-align: left;
    }

    .message {
      display: grid;
      gap: var(--ld-chat-message-gap);
      max-width: min(var(--ld-chat-message-width), 100%);
    }

    .message.user {
      justify-self: end;
    }

    .message.assistant,
    .message.summary,
    .message.error {
      justify-self: start;
    }

    .label {
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-strong);
    }

    .bubble {
      border: var(--ld-border-muted);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-panel);
      padding: var(--ld-chat-bubble-padding-block) var(--ld-chat-bubble-padding-inline);
      font-size: var(--ld-font-size-body-sm);
      line-height: var(--ld-line-height-relaxed);
      overflow-wrap: anywhere;
    }

    .bubble.plain {
      white-space: pre-wrap;
    }

    .bubble.markdown {
      display: block;
    }

    .bubble.markdown :is(p, ul, ol, pre, blockquote) {
      margin-block: 0 var(--ld-chat-markdown-block-gap);
    }

    .bubble.markdown :is(p, ul, ol, pre, blockquote):last-child {
      margin-bottom: 0;
    }

    .bubble.markdown ul,
    .bubble.markdown ol {
      padding-left: var(--ld-chat-markdown-list-indent);
    }

    .bubble.markdown li + li {
      margin-top: var(--ld-chat-markdown-list-item-gap);
    }

    .bubble.markdown code {
      border-radius: var(--ld-chat-code-radius);
      background: var(--ld-bg-control);
      padding: var(--ld-chat-code-padding-block) var(--ld-chat-code-padding-inline);
      font-family: var(--fontStack-monospace);
      font-size: var(--ld-chat-code-font-scale);
    }

    .bubble.markdown pre {
      max-width: 100%;
      overflow: auto;
      border: var(--ld-border-muted);
      border-radius: var(--ld-radius-default);
      background: var(--ld-bg-control);
      padding: var(--ld-chat-pre-padding-block) var(--ld-chat-pre-padding-inline);
    }

    .bubble.markdown pre code {
      border-radius: 0;
      background: transparent;
      padding: 0;
      font-size: var(--ld-font-size-caption);
    }

    .bubble.markdown blockquote {
      border-left: var(--ld-chat-quote-border-width) solid var(--ld-line-muted);
      padding-left: var(--ld-chat-bubble-padding-block);
      color: var(--ld-fg-muted);
    }

    .bubble.markdown a {
      color: var(--ld-fg-accent);
      text-decoration-thickness: var(--ld-chat-link-underline-thickness);
      text-underline-offset: var(--ld-chat-link-underline-offset);
    }

    .user .bubble {
      border-color: var(--ld-line-accent-muted);
      background: var(--ld-bg-accent-muted);
    }

    .message.error .bubble {
      border-color: var(--ld-line-danger-muted);
      background: var(--ld-bg-danger-muted);
    }

    .activity {
      display: flex;
      width: fit-content;
      max-width: 100%;
      align-items: center;
      gap: var(--ld-chat-activity-gap);
      border: var(--ld-border-muted);
      border-radius: var(--ld-radius-full);
      background: var(--ld-bg-panel-muted);
      padding: var(--ld-chat-activity-padding-block) var(--ld-chat-activity-padding-inline);
      color: var(--ld-fg-muted);
      font-size: var(--ld-font-size-caption);
      font-weight: var(--ld-font-weight-strong);
    }

    .dot {
      width: var(--ld-chat-activity-dot-size);
      height: var(--ld-chat-activity-dot-size);
      flex: 0 0 auto;
      border-radius: var(--ld-radius-full);
      background: var(--ld-fg-warning);
    }

    .activity.done .dot {
      background: var(--ld-fg-success);
    }

    .activity.running .dot {
      animation: pulse 1.1s ease-in-out infinite;
    }

    .activity.error .dot {
      background: var(--ld-fg-danger);
    }

    .activity-text {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .activity-detail {
      color: var(--ld-fg-subtle);
      font-weight: var(--ld-font-weight-regular);
    }

    @keyframes pulse {
      0%,
      100% {
        opacity: 0.45;
      }
      50% {
        opacity: 1;
      }
    }

    @media (max-width: 720px) {
      .scroll {
        padding: var(--ld-chat-thread-padding-compact);
      }
    }
  `

  render() {
    const transcript = this.resolvedTranscript

    return html`
      <div class="thread">
        <div class="scroll">
          <div class="stack">
            ${this.status.error ? html`<div class="alert">${this.status.error}</div>` : nothing}
            ${transcript.length === 0
              ? html`<div class="empty">Start a conversation from the composer.</div>`
              : nothing}
            ${transcript.map((item) => this.renderItem(item))}
          </div>
        </div>
      </div>
    `
  }

  private get resolvedTranscript(): ChatTranscriptItem[] {
    return Array.isArray(this.transcript) && this.transcript.length > 0 ? this.transcript : this.transcriptAttribute
  }

  private renderItem(item: ChatTranscriptItem) {
    switch (item.kind) {
      case 'tool':
        return this.renderTool(item)
      case 'user':
        return this.renderMessage('user', 'You', item.text || '-')
      case 'error':
        return this.renderMessage('error', 'Error', item.text || item.error || '-', false, true)
      case 'summary':
        return this.renderMessage('summary', 'Summary', item.markdown || item.text || '-', true)
      case 'assistant':
      default:
        return this.renderMessage('assistant', 'LibreDash', item.markdown || item.text || '-', true)
    }
  }

  private renderMessage(role: string, label: string, content: string, renderMarkdown = false, error = false) {
    return html`
      <article class=${['message', role, error ? 'error' : ''].filter(Boolean).join(' ')}>
        <div class="label">${label}</div>
        ${this.renderBubble(content, renderMarkdown)}
      </article>
    `
  }

  private renderBubble(content: string, renderMarkdown: boolean) {
    return html`<div class=${['bubble', renderMarkdown ? 'markdown' : 'plain'].join(' ')}>${renderMarkdown ? unsafeHTML(renderMarkdownHTML(content)) : content}</div>`
  }

  private renderTool(item: ChatTranscriptItem) {
    const status = item.status || 'running'
    const detail = item.error || item.summary || statusLabel(status)
    return html`
      <div class=${['activity', status === 'running' ? 'running' : '', status === 'complete' ? 'done' : '', status === 'error' ? 'error' : ''].filter(Boolean).join(' ')}>
        <span class="dot" aria-hidden="true"></span>
        <span class="activity-text">${item.title || item.name || 'Tool'}${detail ? html` <span class="activity-detail">${detail}</span>` : nothing}</span>
      </div>
    `
  }

}

function renderMarkdownHTML(value: string): string {
  return DOMPurify.sanitize(markdown.render(value), {
    USE_PROFILES: { html: true },
  })
}

function statusLabel(status: string): string {
  switch (status) {
    case 'complete':
      return 'Complete'
    case 'error':
      return 'Failed'
    case 'streaming':
      return 'Streaming'
    default:
      return 'Running'
  }
}

if (!customElements.get('ld-chat-thread')) customElements.define('ld-chat-thread', ChatThread)
