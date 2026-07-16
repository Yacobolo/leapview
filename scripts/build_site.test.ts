import { expect, test } from 'bun:test'

test('site entrypoint is a production bundle with lazy feature chunks', async () => {
  const entry = Bun.file('site/static/site-page.js')
  expect(await entry.exists()).toBe(true)
  expect(entry.size).toBeLessThan(250_000)

  const source = await entry.text()
  expect(source).not.toContain('Lit is in dev mode')

  const chunks: string[] = []
  const glob = new Bun.Glob('site/static/chunks/*.js')
  for await (const path of glob.scan({ cwd: '.', onlyFiles: true })) chunks.push(path)
  expect(chunks.length).toBeGreaterThanOrEqual(3)
})
