import Ajv from 'ajv'
import addFormats from 'ajv-formats'
import standaloneCode from 'ajv/dist/standalone'

const schemaPath = 'node_modules/vega-lite/build/vega-lite-schema.json'
const outputPath = 'web/generated/vega-lite/validate.ts'
const schema = await Bun.file(schemaPath).json()
const ajv = new Ajv({ allErrors: true, code: { source: true, esm: true, optimize: true }, strict: false })
addFormats(ajv)
ajv.addFormat('color-hex', /^#[0-9a-f]{3}(?:[0-9a-f]{3})?(?:[0-9a-f]{2})?$/i)
const validate = ajv.compile(schema)
const source = standaloneCode(ajv, validate)
await Bun.write(outputPath, `// Code generated from the vega-lite@6.4.3 JSON Schema. DO NOT EDIT.\n// @ts-nocheck\n${source}\n`)
