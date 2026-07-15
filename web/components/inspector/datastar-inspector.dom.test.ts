import { afterAll, beforeAll, expect, test } from 'bun:test'
import { createServer, type Server } from 'node:http'
import { readFile } from 'node:fs/promises'
import { join, normalize } from 'node:path'
import { chromium, type Browser } from '@playwright/test'

let server: Server
let baseURL = ''
let browser: Browser
const traceQueries: string[] = []
const root = join(process.cwd(), '.tmp/datastar-inspector-test')

beforeAll(async () => {
  server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1')
    if (url.pathname === '/') {
      response.setHeader('content-type', 'text/html')
      response.end(testDocument())
      return
    }
    if (url.pathname === '/__dev/pagestream/traces') {
      traceQueries.push(url.search)
      response.setHeader('content-type', 'application/json')
      response.end(JSON.stringify(url.searchParams.get('after') ? { events: [], nextAfter: 2 } : {
        events: [
          {
            id: 1,
            timestamp: '2026-07-14T12:00:00Z',
            streamId: 'dashboard:ratings:tab-1',
            sequence: 1,
            stage: 'published',
            generation: 4,
            origin: 'dashboard.refresh',
            correlationId: 'refresh-4',
            roots: ['status'],
            bytes: 128,
            digest: 'abc123',
            payload: { status: { progressPercent: 0 } },
          },
          {
            id: 2,
            timestamp: '2026-07-14T12:00:00.120Z',
            streamId: 'dashboard:ratings:tab-1',
            sequence: 1,
            stage: 'delivered',
            generation: 4,
            origin: 'dashboard.refresh',
            correlationId: 'refresh-4',
            roots: ['visuals'],
            bytes: 256,
            digest: 'def456',
            queueMilliseconds: 12.5,
            payload: { visuals: { rating_count: { formattedValue: '1.2m' } } },
          },
        ],
        nextAfter: 2,
      }))
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
  if (!address || typeof address === 'string') throw new Error('test server did not bind')
  baseURL = `http://127.0.0.1:${address.port}`
  browser = await chromium.launch()
})

afterAll(async () => {
  await browser?.close()
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
}, 15_000)

test('inspector shows the backend pagestream event timeline and incremental payloads', async () => {
  const page = await browser.newPage({ viewport: { width: 700, height: 600 } })
  try {
    await page.goto(baseURL)
    await page.waitForFunction(() => customElements.get('datastar-inspector'))
    const state = await page.locator('datastar-inspector').evaluate(async (element: any) => {
      element.shadowRoot.querySelector<HTMLButtonElement>('.toggle')!.click()
      await element.updateComplete
      element.shadowRoot.querySelector<HTMLButtonElement>('[data-view="events"]')!.click()
      await element.updateComplete
      const deadline = Date.now() + 3_000
      while (!element.shadowRoot.textContent.includes('dashboard.refresh') && Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 25))
        await element.updateComplete
      }
      const initialText = element.shadowRoot.textContent
      const details = element.shadowRoot.querySelector<HTMLDetailsElement>('.trace-event')!
      details.open = true
      await element.updateComplete
      const payloadText = details.textContent
      const filter = element.shadowRoot.querySelector<HTMLInputElement>('.filter')!
      filter.value = 'visuals'
      filter.dispatchEvent(new Event('input', { bubbles: true }))
      await element.updateComplete
      const filteredText = element.shadowRoot.textContent
      element.shadowRoot.querySelector<HTMLButtonElement>('[data-clear-events]')!.click()
      await element.updateComplete
      const clearedText = element.shadowRoot.textContent
      return { initialText, payloadText, filteredText, clearedText }
    })

    expect(state.initialText).toMatch(/Events/)
    expect(state.initialText).toMatch(/dashboard\.refresh/)
    expect(state.initialText).toMatch(/delivered/)
    expect(state.initialText).toMatch(/12\.5 ms/)
    expect(state.payloadText).toMatch(/progressPercent/)
    expect(state.filteredText).toMatch(/visuals/)
    expect(state.filteredText).not.toMatch(/progress/)
    expect(state.clearedText).toMatch(/No page-stream events/)
    await page.waitForTimeout(600)
    expect(traceQueries.some((query) => query.includes('after=2'))).toBe(true)
  } finally {
    await page.close()
  }
})

function testDocument(): string {
  return `
    <!doctype html>
    <html>
      <body>
        <datastar-inspector trace-url="/__dev/pagestream/traces"></datastar-inspector>
        <script type="module" src="/datastar-inspector-under-test.js"></script>
      </body>
    </html>
  `
}
