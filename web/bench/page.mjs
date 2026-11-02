// The benchmark page: one case per page load, so that the JIT state one case
// leaves behind never favours the next. run.mjs opens it and reads
// window.__done when it is set.
//
//   /?case=<name>&formats=json,colbin&ops=decode,columns,access&rounds=10&minMs=50
//   /?case=<name>&cold=<format label>     one cold start: compile, instantiate, first decode
//
// A sample runs one operation until at least minMs have passed (at least
// once) and records the time per run. Every operation is warmed up once,
// then the samples are taken in rounds, one of each per round, interleaved
// across formats and operations as colbinbench does.

import { formats, cold } from '../lib/formats.mjs'

const params = new URLSearchParams(location.search)

async function main() {
  if (!crossOriginIsolated) {
    throw new Error('the page is not cross-origin isolated: performance.now() would be coarse; serve it with COOP and COEP')
  }
  const index = await (await fetch('/fixtures/index.json')).json()
  const entry = index.cases.find((c) => c.name === params.get('case'))
  if (!entry) throw new Error(`no case ${params.get('case')}`)
  const wasm = new Uint8Array(await (await fetch('/colbin.wasm')).arrayBuffer())
  const env = { wasm }
  const load = async (format) => {
    const fixture = entry.formats[format.name]
    if (!fixture) return null
    const get = async (file) => new Uint8Array(await (await fetch(`/fixtures/${entry.name}/${file}`)).arrayBuffer())
    return {
      bytes: await get(fixture.file),
      files: { schema: fixture.schema ? await get(fixture.schema) : null },
    }
  }

  if (params.has('cold')) {
    const format = formats.find((f) => f.label === params.get('cold'))
    const fixture = await load(format)
    const start = performance.now()
    const value = await cold(format, env, fixture.files, fixture.bytes, entry)
    const ms = performance.now() - start
    if (value === undefined) throw new Error('cold decode returned nothing')
    return { results: [{ format: format.label, op: 'cold', n: 1, ms }] }
  }

  const labels = (params.get('formats') ?? formats.map((f) => f.label).join(',')).split(',')
  const ops = (params.get('ops') ?? 'decode,columns,access').split(',')
  const rounds = Number(params.get('rounds') ?? 10)
  const minMs = Number(params.get('minMs') ?? 50)

  const runners = []
  const paths = {}
  for (const format of formats.filter((f) => labels.includes(f.label))) {
    const fixture = await load(format)
    if (!fixture) continue
    const state = await format.prepare(await format.open(env), fixture.files, entry)
    state.path = entry.access ?? []
    state.tables = entry.tables ?? []
    const { bytes } = fixture
    for (const op of ops) {
      let body
      if (op === 'decode') body = () => format.decode(state, bytes)
      else if (op === 'columns' && format.columns) body = () => format.columns(state, bytes)
      else if (op === 'access' && entry.access) body = () => format.access(state, bytes)
      else continue
      runners.push({ format: format.label, op, body })
    }
    // The warm-up, which is also where a format that throws stops the page
    // before anything is measured. columns() that declines the shape (null)
    // is not timed: there is no column view to time.
    for (const runner of runners.filter((r) => r.format === format.label)) {
      const value = runner.body()
      if (value === null && runner.op === 'columns') runner.skip = true
    }
    if (format.path) {
      format.decode(state, bytes)
      paths[format.label] = format.path(state)
    }
  }

  const results = []
  const gc = globalThis.gc
  let sink = 0
  for (let round = 0; round < rounds; round++) {
    for (const runner of runners) {
      if (runner.skip) continue
      if (gc) gc()
      let n = 0
      const start = performance.now()
      let now = start
      do {
        const value = runner.body()
        if (value !== null && value !== undefined) sink++
        n++
        now = performance.now()
      } while (now - start < minMs)
      results.push({ format: runner.format, op: runner.op, n, ms: now - start })
    }
  }
  return { results, paths, gc: Boolean(gc), sink }
}

main().then(
  (done) => { window.__done = { ok: true, ...done } },
  (err) => { window.__done = { ok: false, error: String(err?.stack ?? err) } },
)
