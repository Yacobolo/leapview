import { LitElement, css, html } from 'lit'

class ReportFooter extends LitElement {
  static styles = css`
    :host {
      display: block;
      min-width: 0;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    footer {
      display: flex;
      min-height: 34px;
      align-items: center;
      justify-content: flex-end;
      border-top: 1px solid var(--borderColor-muted);
      box-sizing: border-box;
      padding: 8px 24px 10px;
    }

    @media (max-width: 860px) {
      footer {
        justify-content: flex-start;
        padding-inline: 12px;
      }
    }
  `

  render() {
    return html`
      <footer part="footer">
        <slot></slot>
      </footer>
    `
  }
}

customElements.define('ld-report-footer', ReportFooter)
