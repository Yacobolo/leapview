import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer, type Server } from 'node:http'
import { readFile } from 'node:fs/promises'
import { join, normalize } from 'node:path'
import { chromium, type Browser } from '@playwright/test'

let server: Server
let baseURL = ''
let browser: Browser

const root = join(process.cwd(), '.tmp/workspace-page-test')

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
  test(`workspace route roots compose UI on ${viewport.name}`, async () => {
    const page = await browser.newPage({ viewport })
    try {
      await page.goto(baseURL)
      await page.waitForFunction(() => (
        customElements.get('ld-workspace-page')
          && customElements.get('ld-connections-page')
          && customElements.get('ld-workspace-asset-page')
          && customElements.get('ld-data-grid')
      ))
      await page.locator('ld-workspace-page').evaluate((element: any) => element.updateComplete)
      await page.locator('ld-connections-page').evaluate((element: any) => element.updateComplete)
      await page.locator('ld-workspace-asset-page').evaluate((element: any) => element.updateComplete)

      const state = await page.evaluate(() => {
        const workspace = document.querySelector('ld-workspace-page') as any
        const connections = document.querySelector('ld-connections-page') as any
        const asset = document.querySelector('ld-workspace-asset-page') as any
        return {
          workspaceTitle: workspace.shadowRoot.querySelector('h1')?.textContent?.trim(),
          workspaceHasAsset: Boolean(workspace.shadowRoot.querySelector('.asset-title')),
          workspaceHasAccess: Boolean(workspace.shadowRoot.querySelector('ld-workspace-access-control')),
          connectionsTitle: connections.shadowRoot.querySelector('h1')?.textContent?.trim(),
          connectionsHasSource: connections.shadowRoot.textContent?.includes('Orders source') ?? false,
          assetTitle: asset.shadowRoot.querySelector('h1 span:last-child')?.textContent?.trim(),
          assetHasOverview: asset.shadowRoot.textContent?.includes('Overview') ?? false,
          assetHasGrid: Boolean(asset.shadowRoot.querySelector('ld-data-grid')),
        }
      })

      assert.deepEqual(state, {
        workspaceTitle: 'LibreDash Workspace',
        workspaceHasAsset: true,
        workspaceHasAccess: true,
        connectionsTitle: 'Connections',
        connectionsHasSource: true,
        assetTitle: 'Olist Commerce',
        assetHasOverview: true,
        assetHasGrid: true,
      })
    } finally {
      await page.close()
    }
  })
}

function testDocument(): string {
  const assetList = {
    workspaceId: 'libredash',
    searchHref: '/workspaces/libredash',
    tabs: [
      { id: '', label: 'All', href: '/workspaces/libredash', active: true },
      { id: 'dashboard', label: 'Dashboard', href: '/workspaces/libredash?type=dashboard', active: false },
    ],
    assets: [{
      id: 'semantic_model:olist',
      title: 'Olist Commerce',
      description: 'Brazilian ecommerce model.',
      type: 'semantic_model',
      typeLabel: 'Semantic model',
      key: 'olist',
      parentTitle: '-',
      detailHref: '/workspaces/libredash/assets/semantic_model:olist/details',
      openHref: '/workspaces/libredash/assets/semantic_model:olist/details',
    }],
    empty: 'No assets match this view.',
  }
  const workspacePage = {
    kind: 'workspace',
    title: 'LibreDash Workspace',
    description: 'Published BI assets.',
    workspaceId: 'libredash',
    assetList,
  }
  const connectionsPage = {
    kind: 'connections',
    title: 'Connections',
    description: 'Connection-scoped data assets.',
    workspaceId: 'libredash',
    assetList: {
      ...assetList,
      searchHref: '/connections',
      assets: [{ ...assetList.assets[0], title: 'Orders source', type: 'source', typeLabel: 'Source', detailHref: '/connections/connection:olist/sources/source:orders/details' }],
    },
  }
  const assetPage = {
    kind: 'workspace_asset',
    title: 'Olist Commerce',
    workspaceId: 'libredash',
    assetId: 'semantic_model:olist',
    activeSection: 'details',
    asset: assetList.assets[0],
    breadcrumbs: [
      { label: 'Workspaces', href: '/workspaces' },
      { label: 'LibreDash Workspace', href: '/workspaces/libredash' },
      { label: 'Olist Commerce', current: true },
    ],
    actions: [],
    tabs: [
      { id: 'details', label: 'Details', href: '/workspaces/libredash/assets/semantic_model:olist/details', active: true },
      { id: 'lineage', label: 'Lineage', href: '/workspaces/libredash/assets/semantic_model:olist/lineage', active: false, count: 1 },
    ],
    details: {
      overview: [
        { label: 'Type', value: 'Semantic model' },
        { label: 'Key', value: 'olist', code: true },
      ],
      sections: [{
        title: 'Model tables (1)',
        grid: {
          columns: [{ id: 'name', header: 'Name', kind: 'link', hrefKey: 'nameHref' }],
          rows: [{ name: 'orders', nameHref: '/workspaces/libredash/assets/model_table:olist.orders/details' }],
          empty: 'No model tables.',
        },
      }],
    },
  }
  const access = {
    workspace: { id: 'libredash', title: 'LibreDash Workspace' },
    roles: [{ name: 'viewer' }],
    bindings: [],
    canManage: true,
    status: { loading: false, error: '', message: '' },
    csrfToken: 'token',
    command: { email: '', role: '', principalId: '' },
    search: '',
  }
  const attr = (value: unknown) => escapeHTML(JSON.stringify(value))
  return `
    <!doctype html>
    <html>
      <head>
        <style>
          html, body { margin: 0; min-height: 100%; }
          body { --fontStack-system: system-ui; --ld-bg-app: #f6f8fa; --ld-bg-panel: #fff; --ld-bg-panel-muted: #f6f8fa; --ld-bg-control: #f6f8fa; --ld-bg-control-hover: #f3f4f6; --ld-fg-default: #24292f; --ld-fg-muted: #57606a; --ld-fg-link: #0969da; --ld-accent: #0969da; --ld-accent-fg: #fff; --ld-line-muted: #d8dee4; --ld-border-default: 1px solid #d0d7de; --ld-border-muted: 1px solid #d8dee4; --ld-border-transparent: 1px solid transparent; --ld-radius-default: 6px; --ld-radius-tight: 4px; --ld-radius-full: 999px; --base-size-4: 4px; --base-size-6: 6px; --base-size-8: 8px; --base-size-10: 10px; --base-size-12: 12px; --base-size-16: 16px; --base-size-20: 20px; --base-size-24: 24px; --control-medium-size: 32px; --control-xlarge-size: 40px; --ld-font-size-caption: 12px; --ld-font-size-body-sm: 14px; --ld-font-size-title-sm: 16px; --ld-font-weight-medium: 500; --ld-font-weight-strong: 600; --ld-line-height-tight: 1.2; --ld-line-height-compact: 1.3; --z-index-inspector: 1000; --ld-modal-backdrop: rgb(0 0 0 / .28); }
          ld-workspace-page, ld-connections-page, ld-workspace-asset-page { display: block; min-height: 720px; }
        </style>
      </head>
      <body>
        <ld-workspace-page page="${attr(workspacePage)}" workspaceaccess="${attr(access)}"></ld-workspace-page>
        <ld-connections-page page="${attr(connectionsPage)}"></ld-connections-page>
        <ld-workspace-asset-page page="${attr(assetPage)}"></ld-workspace-asset-page>
        <script type="module" src="/workspace-page-under-test.js"></script>
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
