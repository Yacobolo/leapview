import { LitElement, html, nothing } from 'lit'
import { property, state } from 'lit/decorators.js'
import { unsafeHTML } from 'lit/directives/unsafe-html.js'
import type { HighlighterCore } from 'shiki/core'

type CodeTheme = 'github-light' | 'github-dark'
type SupportedLanguage = 'json' | 'sql' | 'toon'

let highlighterPromise: Promise<HighlighterCore> | null = null

function loadHighlighter(): Promise<HighlighterCore> {
  highlighterPromise ??= (async () => {
    const [
      { createHighlighterCore },
      { createJavaScriptRegexEngine },
      { default: sql },
      { default: json },
      { default: githubDark },
      { default: githubLight },
      { toonLanguage },
    ] = await Promise.all([
      import('shiki/core'),
      import('shiki/engine/javascript'),
      import('@shikijs/langs/sql'),
      import('@shikijs/langs/json'),
      import('@shikijs/themes/github-dark'),
      import('@shikijs/themes/github-light'),
      import('./toon-language'),
    ])
    return createHighlighterCore({
      themes: [githubLight, githubDark],
      langs: [json, sql, toonLanguage],
      engine: createJavaScriptRegexEngine(),
    })
  })()
  return highlighterPromise
}

class CodeBlock extends LitElement {
  @property({ type: String }) code = ''
  @property({ type: String }) language = 'sql'
  @property({ type: Boolean, reflect: true }) compact = false
  @state() private highlighted = ''
  @state() private error = ''
  private renderToken = 0

  createRenderRoot(): HTMLElement {
    return this
  }

  connectedCallback(): void {
    super.connectedCallback()
    document.addEventListener('libredash-theme-applied', this.handleThemeApplied)
  }

  disconnectedCallback(): void {
    document.removeEventListener('libredash-theme-applied', this.handleThemeApplied)
    super.disconnectedCallback()
  }

  firstUpdated(): void {
    void this.highlight()
  }

  updated(changed: Map<string, unknown>): void {
    if (changed.has('code') || changed.has('language')) {
      void this.highlight()
    }
  }

  render() {
    const code = this.code
    return html`
      <style>
        ${codeBlockStyles}
      </style>
      <div class="code-block-shell">
        ${this.error
          ? html`<pre class="code-block-fallback"><code>${code}</code></pre>`
          : this.highlighted
            ? unsafeHTML(this.highlighted)
            : html`<pre class="code-block-fallback"><code>${code || 'Loading...'}</code></pre>`}
        ${this.error ? html`<p class="code-block-error">${this.error}</p>` : nothing}
      </div>
    `
  }

  private handleThemeApplied = (): void => {
    void this.highlight()
  }

  private async highlight(): Promise<void> {
    const token = ++this.renderToken
    const code = this.code
    const language = supportedLanguage(this.language)
    if (!code.trim() || !language) {
      this.highlighted = ''
      this.error = ''
      return
    }
    try {
      const highlighter = await loadHighlighter()
      if (token !== this.renderToken) return
      this.highlighted = highlighter.codeToHtml(code, {
        lang: language,
        theme: this.theme,
      })
      this.error = ''
    } catch {
      if (token !== this.renderToken) return
      this.highlighted = ''
      this.error = 'Syntax highlighting is unavailable.'
    }
  }

  private get theme(): CodeTheme {
    const colorScheme = document.documentElement.style.colorScheme
    if (colorScheme === 'dark') return 'github-dark'
    return 'github-light'
  }
}

function supportedLanguage(language: string): SupportedLanguage | '' {
  const normalized = language.trim().toLowerCase()
  if (normalized === 'json' || normalized === 'sql' || normalized === 'toon') return normalized
  return ''
}

const codeBlockStyles = `
  ld-code-block {
    display: block;
    min-width: 0;
    max-width: 100%;
  }

  ld-code-block .code-block-shell {
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    border: var(--ld-border-muted);
    border-radius: var(--borderRadius-medium);
    background: var(--ld-bg-panel-muted);
  }

  ld-code-block .shiki,
  ld-code-block .code-block-fallback {
    box-sizing: border-box;
    max-width: 100%;
    max-height: min(44rem, 68vh);
    margin: 0;
    overflow: auto;
    padding: var(--base-size-16);
    font-family: var(--fontStack-monospace, ui-monospace, SFMono-Regular, SFMono-Regular, Consolas, Liberation Mono, monospace);
    font-size: var(--ld-font-size-body-sm);
    line-height: 1.65;
    tab-size: 2;
  }

  ld-code-block[compact] .shiki,
  ld-code-block[compact] .code-block-fallback {
    max-height: var(--ld-chat-tool-max-height, 18rem);
    padding: var(--ld-chat-pre-padding-block, var(--base-size-8)) var(--ld-chat-pre-padding-inline, var(--base-size-12));
    font-size: var(--ld-font-size-caption, 0.75rem);
    line-height: var(--ld-line-height-snug, 1.35);
    white-space: pre;
  }

  ld-code-block .shiki code,
  ld-code-block .code-block-fallback code {
    font-family: inherit;
  }

  ld-code-block .code-block-fallback {
    color: var(--ld-fg-default);
    background: var(--ld-bg-panel-muted);
    white-space: pre;
  }

  ld-code-block .code-block-error {
    margin: 0;
    border-top: var(--ld-border-muted);
    padding: var(--base-size-8) var(--base-size-16);
    color: var(--ld-fg-muted);
    font-size: var(--ld-font-size-caption);
  }
`

if (!customElements.get('ld-code-block')) {
  customElements.define('ld-code-block', CodeBlock)
}
