import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer, type Server } from 'node:http'
import { readFile } from 'node:fs/promises'
import { join, normalize } from 'node:path'
import { chromium, type Browser } from '@playwright/test'

let server: Server
let baseURL = ''
let browser: Browser

const root = join(process.cwd(), '.tmp/admin-page-test')

test.before(async () => {
  server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1')
    if (url.pathname === '/') {
      response.setHeader('content-type', 'text/html')
      response.end(testDocument())
      return
    }
    const file = normalize(join(root, url.pathname))
    if (!file.startsWith(root)) {
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

test.after(async () => {
  await browser?.close()
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
})

for (const viewport of [
  { name: 'desktop', width: 1280, height: 820 },
  { name: 'mobile', width: 390, height: 820 },
]) {
  test(`admin page composes route UI on ${viewport.name}`, async () => {
    const page = await browser.newPage({ viewport })
    try {
      await page.goto(baseURL)
      await page.waitForFunction(() => (
        customElements.get('ld-admin-page')
          && customElements.get('ld-sub-sidebar')
          && customElements.get('ld-data-grid')
      ))
      await page.locator('ld-admin-page').evaluate((element: any) => element.updateComplete)

      const state = await page.locator('ld-admin-page').evaluate((element: any) => {
        const root = element.shadowRoot
        return {
          title: root.querySelector('h1')?.textContent?.trim(),
          hasSidebar: Boolean(root.querySelector('ld-sub-sidebar')),
          hasGrid: Boolean(root.querySelector('ld-data-grid')),
          text: root.textContent,
        }
      })

      assert.equal(state.title, 'Principals')
      assert.equal(state.hasSidebar, true)
      assert.equal(state.hasGrid, true)
      assert.match(state.text ?? '', /analyst@example\.com/)
    } finally {
      await page.close()
    }
  })
}

function testDocument(): string {
  const page = {
    kind: 'admin',
    title: 'Principals',
    active: 'principals',
    sidebar: {
      label: 'Admin',
      railLabel: 'Admin',
      ariaLabel: 'Admin navigation',
      storageKey: 'libredash-admin-sidebar-collapsed',
      activeId: 'principals',
      collapsible: false,
      numbered: false,
      items: [
        { id: 'general', title: 'General', href: '/admin', active: false },
        { id: 'principals', title: 'Principals', href: '/admin/principals', active: true },
        { id: 'groups', title: 'Groups', href: '/admin/groups', active: false },
      ],
    },
    headerTitle: 'Principals',
    headerDetail: 'Users and service principals known to LibreDash.',
    sections: [{
      title: 'Principals',
      grid: {
        columns: [
          { id: 'name', header: 'Name', kind: 'link', hrefKey: 'name_href' },
          { id: 'email', header: 'Email' },
          { id: 'roles', header: 'Direct roles', kind: 'tags' },
        ],
        rows: [{ name: 'Analyst', name_href: '/admin/principals/p1', email: 'analyst@example.com', roles: ['viewer'] }],
        empty: 'No principals found.',
      },
    }],
  }
  const attr = (value: unknown) => escapeHTML(JSON.stringify(value))
  return `
    <!doctype html>
    <html>
      <head>
        <style>
          html, body { margin: 0; min-height: 100%; }
          body { --fontStack-system: system-ui; --ld-bg-app: #f6f8fa; --ld-bg-panel: #fff; --ld-bg-panel-muted: #f6f8fa; --ld-bg-control-hover: #f3f4f6; --ld-fg-default: #24292f; --ld-fg-muted: #57606a; --ld-fg-link: #0969da; --ld-border-default: 1px solid #d0d7de; --ld-border-muted: 1px solid #d8dee4; --ld-radius-default: 6px; --ld-radius-full: 999px; --base-size-4: 4px; --base-size-8: 8px; --base-size-12: 12px; --base-size-16: 16px; --ld-font-size-caption: 12px; --ld-font-size-body-sm: 14px; --ld-font-size-title-sm: 16px; --ld-font-weight-medium: 500; --ld-font-weight-strong: 600; --ld-line-height-tight: 1.2; --ld-line-height-compact: 1.3; }
          ld-admin-page { min-height: 720px; }
        </style>
      </head>
      <body>
        <ld-admin-page page="${attr(page)}"></ld-admin-page>
        <script type="module" src="/admin-page-under-test.js"></script>
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
