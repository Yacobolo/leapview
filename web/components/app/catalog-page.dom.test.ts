import { afterAll, beforeAll, expect, test } from 'bun:test'
import { createServer, type Server } from 'node:http'
import { readFile } from 'node:fs/promises'
import { join, normalize } from 'node:path'
import { chromium, type Browser } from '@playwright/test'

let server: Server
let baseURL = ''
let browser: Browser

const projectRoot = process.cwd()
const root = join(projectRoot, '.tmp/catalog-page-test')

beforeAll(async () => {
  server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1')
    if (url.pathname === '/') {
      response.setHeader('content-type', 'text/html')
      response.end(testDocument())
      return
    }
    const fileRoot = url.pathname.startsWith('/static/vendor/') ? projectRoot : root
    const file = normalize(join(fileRoot, url.pathname))
    if (!file.startsWith(fileRoot)) {
      response.writeHead(404)
      response.end('not found')
      return
    }
    try {
      response.setHeader('content-type', 'text/javascript')
      response.end(await readFile(file))
    } catch {
      response.writeHead(404)
      response.end('not found')
    }
  })
  await new Promise<void>((resolve) => server.listen(0, resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('test server did not bind to a port')
  baseURL = `http://127.0.0.1:${address.port}`
  browser = await chromium.launch()
})

afterAll(async () => {
  await browser?.close()
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
}, 15_000)

for (const viewport of [
  { name: 'compact desktop', width: 706, height: 793 },
  { name: 'mobile', width: 390, height: 820 },
]) {
  test(`catalog page renders compact full-width dashboard rows on ${viewport.name}`, async () => {
    const page = await browser.newPage({ viewport })
    try {
      await page.goto(baseURL)
      await page.waitForFunction(() => customElements.get('ld-catalog-page'))
      await page.locator('ld-catalog-page').evaluate((element: any) => element.updateComplete)

      const state = await page.locator('ld-catalog-page').evaluate((element: any) => {
        const root = element.shadowRoot
        const section = root.querySelector('section') as HTMLElement
        const list = root.querySelector('.dashboard-list') as HTMLElement
        const rows = Array.from(root.querySelectorAll('a.dashboard-row')) as HTMLAnchorElement[]
        const sectionRect = section.getBoundingClientRect()
        const listRect = list.getBoundingClientRect()
        return {
          title: root.querySelector('h1')?.textContent?.trim(),
          rowCount: rows.length,
          hrefs: rows.map((row) => row.getAttribute('href')),
          titles: rows.map((row) => row.querySelector('.dashboard-title')?.textContent?.trim()),
          descriptions: rows.map((row) => row.querySelector('.dashboard-description')?.textContent?.trim()),
          pages: rows.map((row) => row.querySelector('.dashboard-pages')?.textContent?.trim()),
          hasIcons: rows.every((row) => Boolean(row.querySelector('.dashboard-icon svg'))),
          hasChevrons: rows.every((row) => Boolean(row.querySelector('.dashboard-chevron svg'))),
          fullWidth: rows.every((row) => Math.abs(row.getBoundingClientRect().width - listRect.width) <= 1),
          maxRowHeight: Math.max(...rows.map((row) => Math.round(row.getBoundingClientRect().height))),
          totalListHeight: Math.round(listRect.height),
          hasCardGrid: Boolean(root.querySelector('.grid, article')),
          hasOpenLabel: rows.some((row) => row.textContent?.includes('Open')),
          sectionWidth: Math.round(sectionRect.width),
          centeredDelta: Math.round(Math.abs((sectionRect.left + sectionRect.width / 2) - window.innerWidth / 2)),
        }
      })

      expect(state).toEqual({
        title: 'Dashboards',
        rowCount: 2,
        hrefs: ['/dashboards/executive-sales', '/dashboards/operations-health'],
        titles: ['Executive Sales Dashboard', 'Operations Health'],
        descriptions: ['Fixture report', 'Fulfillment and delivery performance.'],
        pages: ['1 page', '3 pages'],
        hasIcons: true,
        hasChevrons: true,
        fullWidth: true,
        maxRowHeight: 72,
        totalListHeight: 144,
        hasCardGrid: false,
        hasOpenLabel: false,
        sectionWidth: Math.min(viewport.width, 1152),
        centeredDelta: 0,
      })
    } finally {
      await page.close()
    }
  })
}

function testDocument(): string {
  const page = {
    kind: 'catalog',
    title: 'Dashboards',
    description: 'Reports backed by semantic models.',
    dashboards: [
      {
        id: 'executive-sales',
        title: 'Executive Sales Dashboard',
        description: 'Fixture report',
        semanticModel: 'olist',
        pageCount: 1,
        tags: ['sales'],
        href: '/dashboards/executive-sales',
      },
      {
        id: 'operations-health',
        title: 'Operations Health',
        description: 'Fulfillment and delivery performance.',
        semanticModel: 'operations',
        pageCount: 3,
        tags: ['operations'],
        href: '/dashboards/operations-health',
      },
    ],
  }
  return `
    <!doctype html>
    <html>
      <head>
        <style>
          html, body { margin: 0; min-height: 100%; }
          body { --fontStack-system: system-ui; --ld-bg-app: #f6f8fa; --ld-bg-panel: #fff; --ld-bg-panel-muted: #f6f8fa; --ld-bg-control-hover: #f3f4f6; --ld-fg-default: #24292f; --ld-fg-muted: #57606a; --ld-fg-link: #0969da; --ld-line-muted: #d8dee4; --ld-line-accent: #0969da; --ld-border-default: 1px solid #d0d7de; --ld-border-muted: 1px solid #d8dee4; --ld-radius-default: 6px; --ld-radius-full: 999px; --ld-page-content-max-width: 72rem; --ld-asset-dashboard-bg: #fbefff; --ld-asset-dashboard-accent: #8250df; --ld-asset-dashboard-border: #d2bfff; --base-size-4: 4px; --base-size-6: 6px; --base-size-8: 8px; --base-size-10: 10px; --base-size-12: 12px; --base-size-16: 16px; --base-size-20: 20px; --borderWidth-default: 1px; --borderWidth-thick: 2px; --control-medium-size: 32px; --ld-font-size-caption: 12px; --ld-font-size-body-sm: 14px; --ld-font-size-body-md: 16px; --ld-font-size-title-sm: 20px; --ld-font-weight-medium: 500; --ld-font-weight-strong: 600; --ld-line-height-tight: 1.1; --ld-line-height-snug: 1.35; --ld-line-height-compact: 1.25; --motion-transition-stateChange: 160ms ease; }
        </style>
      </head>
      <body>
        <main data-signals="${escapeHTML(JSON.stringify({ page }))}">
          <ld-catalog-page></ld-catalog-page>
        </main>
        <script type="module" src="/catalog-page-under-test.js"></script>
        <script type="module" src="/static/vendor/datastar-1.0.2.js?v=dev"></script>
      </body>
    </html>
  `
}

function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}
