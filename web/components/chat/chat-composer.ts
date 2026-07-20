import { LitElement, css, html } from 'lit'
import { property, state } from 'lit/decorators.js'
import { Send } from 'lucide'
import type { AgentVisualReferenceSignal } from '../../generated/signals'
import { lucideIcon } from '../shared/lucide-icons'

export type ChatContextReference = AgentVisualReferenceSignal

class ChatComposer extends LitElement {
  @property({ type: String }) value = ''
  @property({ type: Boolean, reflect: true }) disabled = false
  @property({ type: Boolean, reflect: true }) pending = false
  @property({ type: String }) placeholder = 'Ask about dashboards, metrics, or models...'
	@property({ attribute: false }) references: ChatContextReference[] = []
	@property({ attribute: false }) suggestions: ChatContextReference[] = []
  @state() private draft = ''
	@state() private mentionIndex = 0
  private resizeObserver?: ResizeObserver
  private observedWidth = -1

  static styles = css`
    :host {
      position: relative;
      display: block;
      background: linear-gradient(to bottom, transparent, var(--ld-bg-app) var(--ld-space-lg));
      color: var(--ld-fg-default);
      font-family: var(--fontStack-system);
    }

    form {
      width: min(calc(100% - var(--ld-space-lg) - var(--ld-space-lg)), var(--ld-chat-stack-width));
      margin-inline: auto;
      padding: calc(var(--ld-space-lg) + var(--ld-space-sm)) var(--ld-space-lg) var(--ld-space-lg);
    }

    .composer-surface {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: end;
      gap: var(--ld-space-sm);
      border: var(--ld-border-muted);
      border-radius: var(--ld-radius-large);
      background: var(--ld-bg-panel);
      padding: var(--ld-space-sm);
      box-shadow: none;
      transition:
        background var(--ld-transition-fast),
        border-color var(--ld-transition-fast),
        box-shadow var(--ld-transition-fast);
    }

    .composer-surface:hover:not(.is-disabled) {
      border-color: var(--ld-line-muted);
      box-shadow: none;
    }

    .composer-surface:focus-within {
      border-color: var(--ld-line-accent-muted);
      box-shadow: 0 0 0 var(--ld-border-width-focus) var(--ld-bg-accent-muted);
    }

    .composer-surface.is-disabled {
      background: var(--ld-bg-control);
      color: var(--ld-fg-muted);
      box-shadow: none;
    }

    textarea {
      box-sizing: border-box;
      min-height: var(--ld-control-medium);
      max-height: 160px;
      width: 100%;
      grid-column: 1;
      grid-row: 1;
      resize: none;
      overflow-y: auto;
      border: 0;
      border-radius: calc(var(--ld-radius-default) - var(--ld-space-2xs));
      background: transparent;
      color: var(--ld-fg-default);
      font: inherit;
      font-size: var(--ld-font-size-body-sm);
      line-height: var(--ld-line-height-normal);
      padding: var(--ld-space-xs) var(--ld-space-sm);
      outline: 0;
    }

    textarea:focus {
      outline: 0;
    }

    textarea::placeholder {
      color: var(--ld-fg-muted);
    }

    .actions {
      display: flex;
      grid-column: 2;
      grid-row: 1;
      min-height: var(--ld-control-medium);
      align-items: center;
      justify-content: flex-end;
    }

		.mention-picker {
			display: grid;
			grid-column: 1 / -1;
			grid-row: 2;
			max-height: 180px;
			overflow: auto;
			border-top: var(--ld-border-muted);
			padding-top: var(--ld-space-sm);
		}

		.mention-option {
			display: grid;
			width: 100%;
			height: auto;
			min-height: var(--ld-control-medium);
			grid-template-columns: minmax(0, 1fr) auto;
			gap: var(--ld-space-sm);
			border: 0;
			border-radius: var(--ld-radius-default);
			background: transparent;
			color: var(--ld-fg-default);
			padding: var(--ld-space-sm);
			box-shadow: none;
			text-align: left;
		}

		.mention-option[data-active='true'],
		.mention-option:hover {
			background: var(--ld-bg-control-hover);
			transform: none;
		}

		.mention-kind {
			color: var(--ld-fg-muted);
			font-size: var(--ld-font-size-caption);
			font-weight: var(--ld-font-weight-medium);
			text-transform: capitalize;
		}

    button {
      display: inline-flex;
      width: var(--ld-button-height, var(--ld-control-medium));
      height: var(--ld-button-height, var(--ld-control-medium));
      min-width: var(--ld-button-height, var(--ld-control-medium));
      align-items: center;
      justify-content: center;
      border: var(--borderWidth-default, var(--ld-border-width)) solid var(--ld-button-accent-border-rest, var(--ld-accent));
      border-radius: var(--ld-button-radius, var(--ld-radius-default));
      background: var(--ld-button-accent-bg-rest, var(--ld-accent));
      color: var(--ld-button-accent-fg-rest, var(--ld-accent-fg));
      cursor: pointer;
      font: inherit;
      font-size: var(--ld-font-size-body-sm);
      font-weight: var(--ld-font-weight-strong);
      padding: 0;
      box-shadow: var(--ld-button-shadow-resting, var(--shadow-resting-small));
      transition:
        background var(--duration-fast) var(--ease-ld),
        border-color var(--duration-fast) var(--ease-ld),
        color var(--duration-fast) var(--ease-ld),
        transform var(--duration-fast) var(--ease-ld);
    }

    button svg {
      width: var(--ld-button-icon-size, var(--base-size-16));
      height: var(--ld-button-icon-size, var(--base-size-16));
    }

    button:hover:not(:disabled) {
      border-color: var(--ld-button-accent-border-hover, var(--ld-accent));
      background: var(--ld-button-accent-bg-hover, var(--ld-accent));
      transform: translateY(-1px);
    }

    button:focus-visible {
      outline: var(--focus-outline, var(--ld-border-default));
      outline-color: var(--borderColor-accent-emphasis, var(--ld-line-accent));
      outline-offset: var(--focus-outline-offset, var(--ld-space-xs));
    }

    .spinner {
      width: 14px;
      height: 14px;
      border: var(--borderWidth-thick) solid transparent;
      border-top-color: currentColor;
      border-radius: 999px;
      animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }

    button:disabled {
      border-color: var(--ld-button-accent-border-disabled, var(--ld-line-default));
      background: var(--ld-button-accent-bg-disabled, var(--ld-bg-control));
      color: var(--ld-button-accent-fg-disabled, var(--ld-fg-muted));
      cursor: not-allowed;
      opacity: 1;
      box-shadow: none;
    }

    textarea:disabled {
      cursor: not-allowed;
      color: var(--ld-fg-muted);
      opacity: 1;
    }
    @media (max-width: 560px) {
      form {
        width: min(calc(100% - var(--ld-space-md) - var(--ld-space-md)), var(--ld-chat-stack-width));
        padding: calc(var(--ld-space-lg) + var(--ld-space-sm)) var(--ld-space-md) var(--ld-space-md);
      }
    }
  `

  updated(changed: Map<string, unknown>) {
    if (changed.has('value')) {
      this.draft = this.value || ''
      void this.updateComplete.then(() => this.resizeTextarea())
    }
  }

  connectedCallback() {
    super.connectedCallback()
    this.draft = this.value || ''
  }

  protected firstUpdated() {
    this.resizeTextarea()
    this.resizeObserver = new ResizeObserver(([entry]) => {
      const width = Math.round(entry?.contentRect.width ?? 0)
      if (width === this.observedWidth) return
      this.observedWidth = width
      this.resizeTextarea()
    })
    this.resizeObserver.observe(this)
  }

  disconnectedCallback() {
    this.resizeObserver?.disconnect()
    this.resizeObserver = undefined
    super.disconnectedCallback()
  }

  public remeasure(): void {
    this.resizeTextarea()
  }

  render() {
    const blocked = this.disabled || this.pending
		const mentions = this.mentionSuggestions()
    return html`
      <form @submit=${this.submit}>
        <div class=${['composer-surface', blocked ? 'is-disabled' : ''].filter(Boolean).join(' ')}>
          <textarea
            .value=${this.draft}
            ?disabled=${this.disabled}
            placeholder=${this.placeholder}
            rows="1"
            @input=${this.input}
            @keydown=${this.keydown}
          ></textarea>
					${mentions.length > 0 ? html`
						<div class="mention-picker" role="listbox" aria-label="Reference a visual">
							${mentions.map((reference, index) => html`
								<button
									type="button"
									class="mention-option"
									role="option"
									aria-selected=${String(index === this.mentionIndex)}
									data-active=${String(index === this.mentionIndex)}
									@mousedown=${(event: MouseEvent) => event.preventDefault()}
									@click=${() => this.selectMention(reference)}
								>
									<span>${reference.title}</span>
									<span class="mention-kind">${reference.visualType}</span>
								</button>
							`)}
						</div>
					` : null}
          <div class="actions">
            <button
              type="submit"
              aria-label=${this.pending ? 'Sending' : 'Send'}
              title="Send"
              ?disabled=${this.disabled || this.pending || this.draft.trim() === ''}
            >
              ${this.pending ? html`<span class="spinner" aria-hidden="true"></span>` : lucideIcon(Send)}
            </button>
          </div>
        </div>
      </form>
    `
  }

  private input(event: Event) {
    const textarea = event.target as HTMLTextAreaElement
    this.draft = textarea.value
		this.mentionIndex = 0
    this.resizeTextarea(textarea)
  }

  private keydown(event: KeyboardEvent) {
		const mentions = this.mentionSuggestions()
		if (mentions.length > 0) {
			if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
				event.preventDefault()
				const direction = event.key === 'ArrowDown' ? 1 : -1
				this.mentionIndex = (this.mentionIndex + direction + mentions.length) % mentions.length
				return
			}
			if (event.key === 'Enter' && !event.shiftKey) {
				event.preventDefault()
				this.selectMention(mentions[this.mentionIndex] ?? mentions[0])
				return
			}
			if (event.key === 'Escape') {
				event.preventDefault()
				this.draft = this.draft.replace(/@[^@\s]*$/, '')
				return
			}
		}
    if (event.key !== 'Enter' || event.shiftKey) return
    event.preventDefault()
    this.dispatchSubmit()
  }

  private submit(event: Event) {
    event.preventDefault()
    this.dispatchSubmit()
  }

  private dispatchSubmit() {
    const input = this.draft.trim()
    if (this.disabled || this.pending || input === '') return
    this.dispatchEvent(new CustomEvent('ld-chat-submit', {
      bubbles: true,
      composed: true,
			detail: { input, references: this.references },
    }))
  }

	private mentionSuggestions(): ChatContextReference[] {
		const match = this.draft.match(/(?:^|\s)@([^@\s]*)$/)
		if (!match) return []
		const query = (match[1] ?? '').toLocaleLowerCase()
		const selected = new Set(this.references.map((reference) => reference.componentId))
		return this.suggestions
			.filter((reference) => !selected.has(reference.componentId))
			.filter((reference) => `${reference.title} ${reference.visualType}`.toLocaleLowerCase().includes(query))
			.slice(0, 8)
	}

	private selectMention(reference: ChatContextReference | undefined) {
		if (!reference) return
		this.draft = this.draft.replace(/@[^@\s]*$/, '').replace(/\s+$/, '')
		this.references = [...this.references, reference]
		this.mentionIndex = 0
		this.dispatchEvent(new CustomEvent('ld-chat-references-change', {
			bubbles: true,
			composed: true,
			detail: { references: this.references },
		}))
		void this.updateComplete.then(() => this.shadowRoot?.querySelector('textarea')?.focus())
	}

  private resizeTextarea(textarea = this.shadowRoot?.querySelector('textarea') as HTMLTextAreaElement | null) {
    if (!textarea) return
    if (textarea.getBoundingClientRect().width <= 0) {
      textarea.style.height = ''
      return
    }
    const maxHeight = 160
    textarea.style.height = 'auto'
    const height = Math.min(textarea.scrollHeight, maxHeight)
    textarea.style.height = `${height}px`
    textarea.style.overflowY = textarea.scrollHeight > maxHeight ? 'auto' : 'hidden'
  }
}

if (!customElements.get('ld-chat-composer')) customElements.define('ld-chat-composer', ChatComposer)
