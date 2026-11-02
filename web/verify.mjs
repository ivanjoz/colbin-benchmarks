#!/usr/bin/env node
// Checks, before anything is measured, that every format decodes every web
// fixture to the same value as the Go JSON of the same case (lib/equal.mjs
// says what "the same" means), and that its access aggregate is the one Go
// computed. A format that cannot carry a case is n/a with the reason, as in Go.
//
//   node verify.mjs                       every case in fixtures/index.json
//   node verify.mjs --cases 'publicbi/*'  some
//   node verify.mjs --fixtures DIR
//
// Exits non-zero if any format that has a fixture fails.

import { readFileSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'
import { parseArgs } from 'node:util'

import { formats } from './lib/formats.mjs'
import { compare, parseReference } from './lib/equal.mjs'
import { selectCases } from './lib/cases.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const { values: args } = parseArgs({
  options: {
    cases: { type: 'string', default: 'all' },
    fixtures: { type: 'string', default: join(here, 'fixtures') },
    formats: { type: 'string', default: formats.map((f) => f.label).join(',') },
  },
})

const index = JSON.parse(readFileSync(join(args.fixtures, 'index.json'), 'utf8'))
const wasm = readFileSync(createRequire(import.meta.url).resolve('colbin/colbin.wasm'))
const env = { wasm }
const wanted = new Set(args.formats.split(','))
const chosen = formats.filter((f) => wanted.has(f.label))
const handles = new Map()
for (const format of chosen) handles.set(format, await format.open(env))

let failures = 0
const paths = new Map()
for (const entry of selectCases(index.cases, args.cases)) {
  const dir = join(args.fixtures, entry.name)
  const read = (file) => new Uint8Array(readFileSync(join(dir, file)))
  const reference = parseReference(new TextDecoder().decode(read('json.json')))
  const line = []
  for (const format of chosen) {
    const fixture = entry.formats[format.name]
    if (!fixture) {
      line.push(`${format.label}: n/a (${entry.na?.[format.name] ?? 'no fixture'})`)
      continue
    }
    const files = { schema: fixture.schema ? read(fixture.schema) : null }
    const bytes = read(fixture.file)
    try {
      const state = await format.prepare(handles.get(format), files, entry)
      state.path = entry.access ?? []
      state.tables = entry.tables ?? []
      const got = format.decode(state, bytes)
      const diff = compare(got, reference, format.key)
      if (diff) throw new Error(`differs from JSON: ${diff}`)
      let note = format.path ? ` (${format.path(state)})` : ''
      if (format.path) paths.set(`${format.label} ${format.path(state)}`, (paths.get(`${format.label} ${format.path(state)}`) ?? 0) + 1)
      if (entry.access) {
        const sum = format.access(state, bytes)
        const want = entry.accessSum
        if (Math.abs(sum - want) > 1e-9 * Math.max(1, Math.abs(want))) throw new Error(`access aggregate ${sum}, want ${want}`)
      }
      line.push(`${format.label}: ok${note}`)
    } catch (err) {
      failures++
      line.push(`${format.label}: FAIL ${err.message}`)
    }
  }
  console.log(`${entry.name}\n  ${line.join('\n  ')}`)
}
console.log()
for (const [path, count] of paths) console.log(`${path}: ${count} cases`)
if (failures) {
  console.error(`\n${failures} failures`)
  process.exit(1)
}
console.log('\nevery fixture decodes to the Go JSON value')
