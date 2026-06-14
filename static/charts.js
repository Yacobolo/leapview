import { LitElement, css, html, svg } from 'https://cdn.jsdelivr.net/npm/lit@3/+esm';

const chartStyles = css`
  :host {
    display: block;
    height: 100%;
    min-height: 286px;
    color: var(--fgColor-default);
    font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  }

  .chart {
    display: grid;
    grid-template-rows: auto 1fr;
    height: 100%;
    min-height: 286px;
    background: var(--bgColor-default);
  }

  header {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    align-items: baseline;
    min-height: 42px;
    border-bottom: 1px solid var(--borderColor-default);
    padding: 10px 12px 8px;
  }

  h2 {
    margin: 0;
    font-size: 0.98rem;
    font-weight: 850;
    letter-spacing: 0;
  }

  .unit {
    color: var(--fgColor-muted);
    font-size: 0.72rem;
    font-weight: 900;
    text-transform: uppercase;
  }

  .empty {
    display: grid;
    place-items: center;
    margin: 12px;
    min-height: 210px;
    border: 1px dashed var(--borderColor-default);
    background: var(--bgColor-muted);
    color: var(--fgColor-muted);
    font-weight: 800;
  }

  svg {
    width: 100%;
    height: 224px;
    padding: 12px;
    overflow: visible;
    box-sizing: border-box;
  }

  text {
    fill: var(--fgColor-muted);
    font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-size: 10px;
    font-weight: 750;
  }
`;

class BaseChart extends LitElement {
  static properties = {
    data: { type: Array },
    chartTitle: { type: String, attribute: 'chart-title' },
    unit: { type: String },
  };

  constructor() {
    super();
    this.data = [];
    this.chartTitle = 'Chart';
    this.unit = '';
  }
}

function format(value) {
  if (!Number.isFinite(value)) return '-';
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}m`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
}

class LineChart extends BaseChart {
  static styles = chartStyles;

  render() {
    const data = this.data ?? [];
    const max = Math.max(...data.map((d) => d.value), 1);
    const width = 760;
    const height = 224;
    const pad = 28;
    const step = data.length > 1 ? (width - pad * 2) / (data.length - 1) : 0;
    const points = data.map((d, index) => {
      const x = pad + index * step;
      const y = height - pad - (d.value / max) * (height - pad * 2);
      return { ...d, x, y };
    });
    const path = points.map((p, index) => `${index === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');
    const area = points.length ? `${path} L${points.at(-1).x},${height - pad} L${points[0].x},${height - pad} Z` : '';

    return html`
      <section class="chart">
        <header>
          <h2>${this.chartTitle ?? 'Chart'}</h2>
          <span class="unit">${this.unit ?? ''}</span>
        </header>
        ${data.length === 0 ? html`<div class="empty">Waiting for signal data</div>` : svg`
          <svg viewBox="0 0 ${width} ${height}" role="img" aria-label=${this.chartTitle ?? 'Line chart'}>
            <path d=${area} fill="color-mix(in srgb, var(--data-blue-color-emphasis), transparent 86%)"></path>
            <path d=${path} fill="none" stroke="var(--data-blue-color-emphasis)" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"></path>
            ${points.map((p) => svg`<circle cx=${p.x} cy=${p.y} r="3.6" fill="var(--bgColor-default)" stroke="var(--data-blue-color-emphasis)" stroke-width="2.4"><title>${p.label}: ${format(p.value)}</title></circle>`)}
            ${points.filter((_, index) => index === 0 || index === points.length - 1 || index % Math.ceil(points.length / 6) === 0).map((p) => svg`<text x=${p.x} y=${height - 4} text-anchor="middle">${p.label}</text>`)}
          </svg>
        `}
      </section>
    `;
  }
}

class BarChart extends BaseChart {
  static styles = chartStyles;

  render() {
    const data = this.data ?? [];
    const max = Math.max(...data.map((d) => d.value), 1);
    const width = 760;
    const rowHeight = 28;
    const height = Math.max(230, data.length * rowHeight + 32);

    return html`
      <section class="chart">
        <header>
          <h2>${this.chartTitle ?? 'Chart'}</h2>
          <span class="unit">${this.unit ?? ''}</span>
        </header>
        ${data.length === 0 ? html`<div class="empty">Waiting for signal data</div>` : svg`
          <svg viewBox="0 0 ${width} ${height}" role="img" aria-label=${this.chartTitle ?? 'Bar chart'}>
            ${data.map((d, index) => {
              const y = 14 + index * rowHeight;
              const barWidth = Math.max(2, (d.value / max) * 470);
              const tone = index % 4 === 0 ? 'var(--data-blue-color-emphasis)' : index % 4 === 1 ? 'var(--data-green-color-emphasis)' : index % 4 === 2 ? 'var(--data-purple-color-emphasis)' : 'var(--data-coral-color-emphasis)';
              return svg`
                <text x="0" y=${y + 16}>${d.label}</text>
                <rect x="210" y=${y} width=${barWidth} height="16" rx="1.5" fill=${tone}></rect>
                <text x=${220 + barWidth} y=${y + 15}>${format(d.value)}</text>
              `;
            })}
          </svg>
        `}
      </section>
    `;
  }
}

class KPIStrip extends LitElement {
  static properties = {
    items: { type: Array },
  };

  constructor() {
    super();
    this.items = [];
  }

  static styles = css`
    :host {
      display: block;
    }

    .strip {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }

    .kpi {
      position: relative;
      min-height: 104px;
      border: 1px solid var(--borderColor-default);
      border-radius: 6px;
      background: var(--bgColor-default);
      box-shadow: var(--shadow-resting-small);
      padding: 12px 14px 12px 16px;
      overflow: hidden;
    }

    .kpi::before {
      content: '';
      position: absolute;
      inset-block: 0;
      left: 0;
      width: 5px;
      background: var(--borderColor-muted);
    }

    .label {
      color: var(--fgColor-muted);
      font-size: 0.72rem;
      font-weight: 900;
      text-transform: uppercase;
    }

    .value {
      margin: 8px 0 4px;
      font-size: clamp(1.72rem, 3.5vw, 2.65rem);
      font-weight: 850;
      line-height: 1;
      letter-spacing: 0;
    }

    .note {
      color: var(--fgColor-muted);
      font-size: 0.85rem;
      font-weight: 700;
    }

    .green::before { background: var(--fgColor-success); }
    .amber::before { background: var(--fgColor-attention); }
    .coral::before { background: var(--fgColor-danger); }
    .ink::before { background: var(--data-blue-color-emphasis); }
    .neutral::before { background: var(--borderColor-muted); }

    @media (max-width: 760px) {
      .strip {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    @media (max-width: 440px) {
      .strip {
        grid-template-columns: 1fr;
      }
    }
  `;

  render() {
    const kpis = this.items ?? [];
    return html`
      <section class="strip" aria-label="Key metrics">
        ${(kpis.length ? kpis : [{ label: 'Orders', value: '-', note: 'Waiting for stream', tone: 'neutral' }]).map((item) => html`
          <article class="kpi ${item.tone ?? 'neutral'}">
            <div class="label">${item.label}</div>
            <div class="value">${item.value}</div>
            <div class="note">${item.note}</div>
          </article>
        `)}
      </section>
    `;
  }
}

customElements.define('ld-line-chart', LineChart);
customElements.define('ld-bar-chart', BarChart);
customElements.define('ld-kpi-strip', KPIStrip);
