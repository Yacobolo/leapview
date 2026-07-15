const result = await Bun.build({
  entrypoints: ['site/web/site-page.ts'],
  target: 'browser',
  format: 'esm',
  outdir: 'site/static',
  naming: { entry: '[name].[ext]' },
})

for (const log of result.logs) {
  console.error(log)
}
if (!result.success) {
  throw new Error('failed to build LibreDash site assets')
}
