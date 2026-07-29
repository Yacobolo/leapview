import { expect, test } from 'bun:test'
import { readFile } from 'node:fs/promises'

test('UI framework QA gives the managed dev task its full readiness budget', async () => {
  const source = await readFile('scripts/qa_ui_framework.ts', 'utf8')

  expect(source).toContain("LEAPVIEW_DEV_READY_ATTEMPTS: String(managedServerReadyAttempts)")
})
