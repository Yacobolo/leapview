import { datastarRuntimeURL } from '../web/components/shared/datastar-runtime'

await Bun.$`rm -rf site/static/site-page.js site/static/chunks site/static/shared site/static/vendor`.quiet()
await Bun.$`mkdir -p site/static/shared site/static/vendor`.quiet()
await Promise.all([
  Bun.write('site/static/shared/app.css', Bun.file('static/app.css')),
  Bun.write('site/static/shared/theme.js', Bun.file('static/theme.js')),
  Bun.write('site/static/vendor/datastar-1.0.2.js', Bun.file('static/vendor/datastar-1.0.2.js')),
])

const result = await Bun.build({
  entrypoints: ['site/web/site-page.ts'],
  target: 'browser',
  format: 'esm',
  splitting: true,
  minify: true,
  define: { 'process.env.NODE_ENV': '"production"' },
  external: [datastarRuntimeURL],
  outdir: 'site/static',
  naming: { entry: '[name].[ext]', chunk: 'chunks/[name]-[hash].[ext]' },
})

for (const log of result.logs) {
  console.error(log)
}
if (!result.success) {
  throw new Error('failed to build LibreDash site assets')
}

const entry = Bun.file('site/static/site-page.js')
if (entry.size >= 250_000) {
  throw new Error(`site entrypoint is ${entry.size} bytes; budget is 250000 bytes`)
}
