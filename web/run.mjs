#!/usr/bin/env node
// The browser benchmark's runner: serves the page and the fixtures, drives
// Chromium, Firefox and WebKit through Playwright, and writes Go benchmark
// format, so the results summarise with benchstat like the Go matrix's.
//
//   node run.mjs                                  chromium, every web case
//   node run.mjs --engines chromium,firefox,webkit
//   node run.mjs --cases 'publicbi/*' --formats json,colbin
//   node run.mjs --out ../results/local/<date>    (bench/local.sh --all passes it)
//
// One page per case, so the JIT state a case leaves never favours the next;
// a cold start is one page per format and round. The browser's processes are
// pinned to --cores (taskset), one hardware thread per physical performance
// core; this process and its HTTP server run on the other CPUs.
//
// Output: <out>/bench-browser-<engine>.txt, with the lines
//
//   BenchmarkWeb/<case>/<format>/<op>  N  X ns/op  Y ns/row
//
// after configuration lines (browser, engine, regime, cpu), and
// <out>/browser-paths-<engine>.csv: which path colbin's decode took per case.

import { createServer } from 'node:http'
import { readFile, mkdir, writeFile, appendFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { join, dirname, normalize, extname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'
import { availableParallelism, cpus } from 'node:os'
import { parseArgs } from 'node:util'
import * as playwright from 'playwright'

import { bundle } from './bench/build.mjs'
import { formats } from './lib/formats.mjs'
import { selectCases } from './lib/cases.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const { values: args } = parseArgs({
  options: {
    engines: { type: 'string', default: 'chromium' },
    cases: { type: 'string', default: 'all' },
    formats: { type: 'string', default: formats.map((f) => f.label).join(',') },
    ops: { type: 'string', default: 'decode,columns,access,cold' },
    rounds: { type: 'string', default: '10' },
    'min-ms': { type: 'string', default: '50' },
    cores: { type: 'string', default: '0,2' },
    fixtures: { type: 'string', default: join(here, 'fixtures') },
    out: { type: 'string', default: join(here, '..', 'results', 'local', stamp()) },
    headed: { type: 'boolean', default: false },
  },
})

const ENGINES = { chromium: 'v8', firefox: 'spidermonkey', webkit: 'javascriptcore' }
const rounds = Number(args.rounds)
const ops = args.ops.split(',')
const labels = args.formats.split(',')
const measured = args.cores.split(',').map(Number)

function stamp() {
  const d = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`
}

// ---- pinning ------------------------------------------------------------

/** Pins every thread of pid and of its descendants to cpus. Processes the
 * browser starts later inherit the affinity of the one that starts them. */
function pinTree(pid, cpuList) {
  const all = [pid]
  for (let i = 0; i < all.length; i++) {
    try {
      const children = execFileSync('pgrep', ['-P', String(all[i])], { encoding: 'utf8' })
      all.push(...children.split('\n').filter(Boolean).map(Number))
    } catch {} // pgrep exits 1 when there are none
  }
  for (const p of all) {
    try {
      execFileSync('taskset', ['-a', '-p', '-c', cpuList, String(p)], { stdio: 'ignore' })
    } catch {} // a process that exited meanwhile
  }
  return all.length
}

// This process and the server keep off the measured cores.
const others = [...Array(availableParallelism()).keys()].filter((c) => !measured.includes(c))
if (others.length > 0) pinTree(process.pid, others.join(','))

// ---- the server -----------------------------------------------------------

const wasmPath = createRequire(import.meta.url).resolve('colbin/colbin.wasm')
const pagePath = await bundle()
const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.json': 'application/json', '.wasm': 'application/wasm' }

const server = createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost')
  let file
  if (url.pathname === '/') file = join(here, 'bench', 'index.html')
  else if (url.pathname === '/page.js') file = pagePath
  else if (url.pathname === '/colbin.wasm') file = wasmPath
  else if (url.pathname.startsWith('/fixtures/')) {
    const rel = normalize(decodeURIComponent(url.pathname.slice('/fixtures/'.length)))
    if (rel.startsWith('..')) return void res.writeHead(403).end()
    file = join(args.fixtures, rel)
  } else return void res.writeHead(404).end()
  try {
    const body = await readFile(file)
    res.writeHead(200, {
      'Content-Type': TYPES[extname(file)] ?? 'application/octet-stream',
      // Cross-origin isolation, for a performance.now() of about 5 µs rather
      // than 100 µs; the page refuses to run without it.
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'require-corp',
      'Cross-Origin-Resource-Policy': 'same-origin',
      'Cache-Control': 'no-store',
    })
    res.end(body)
  } catch {
    res.writeHead(404).end()
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const origin = `http://127.0.0.1:${server.address().port}`

// ---- the run --------------------------------------------------------------

const index = JSON.parse(await readFile(join(args.fixtures, 'index.json'), 'utf8'))
const cases = selectCases(index.cases, args.cases).filter((c) => c.regime === 'web')
if (cases.length === 0) throw new Error(`no web case matches ${args.cases}`)
await mkdir(args.out, { recursive: true })

const cpuName = cpus()[0]?.model?.trim() ?? 'unknown'
const warmOps = ops.filter((op) => op !== 'cold').join(',')

for (const engine of args.engines.split(',')) {
  const type = playwright[engine]
  if (!type || !ENGINES[engine]) throw new Error(`no engine ${engine}`)
  // --expose-gc lets the page collect before every sample, as the Go matrix
  // does; only V8 offers it, and the output says whether it was used.
  const launchArgs = engine === 'chromium' ? ['--js-flags=--expose-gc'] : []
  let browserServer
  try {
    // Chromium's full browser in its new headless mode, not the separate
    // headless shell Playwright uses by default: the browser a user runs.
    const channel = engine === 'chromium' ? { channel: 'chromium' } : {}
    browserServer = await type.launchServer({ headless: !args.headed, args: launchArgs, ...channel })
  } catch (err) {
    // Reported, not skipped silently: a missing engine is a result too.
    const reason = err.message.split("\n").map((l) => l.replace(/[║╔╗╚╝═]/g, "").trim()).filter(Boolean).slice(1, 6).join(" ")
    console.error(`${engine}: not available: ${reason}`)
    await writeFile(join(args.out, `bench-browser-${engine}.txt`), `# ${engine}: not available: ${reason}\n`)
    continue
  }
  const browser = await type.connect(browserServer.wsEndpoint())
  const browserPid = browserServer.process().pid
  const file = join(args.out, `bench-browser-${engine}.txt`)
  const pathsFile = join(args.out, `browser-paths-${engine}.csv`)
  await writeFile(file,
    `goos: browser\ngoarch: ${process.arch}\npkg: web\ncpu: ${cpuName}\n` +
    `browser: ${engine} ${browser.version()}\nengine: ${ENGINES[engine]}\nregime: web\n` +
    `pinned: ${args.cores}\n`)
  await writeFile(pathsFile, 'case,format,path\n')
  console.error(`${engine} ${browser.version()}: ${cases.length} cases, browser pinned to ${args.cores}`)

  const open = async (query) => {
    pinTree(browserPid, args.cores)
    const context = await browser.newContext()
    const page = await context.newPage()
    pinTree(browserPid, args.cores) // the renderer this page started
    try {
      await page.goto(`${origin}/?${query}`)
      const handle = await page.waitForFunction(() => window.__done, null, { timeout: 0, polling: 100 })
      const done = await handle.jsonValue()
      if (!done.ok) throw new Error(done.error)
      return done
    } finally {
      await context.close()
    }
  }

  let gcNoted = false
  for (const entry of cases) {
    const lines = []
    const add = (r) => {
      const ns = (r.ms * 1e6) / r.n
      lines.push(`BenchmarkWeb/${entry.name}/${r.format}/${r.op} ${r.n} ${ns.toFixed(1)} ns/op ${(ns / entry.rows).toFixed(2)} ns/row`)
    }
    if (warmOps) {
      const query = new URLSearchParams({ case: entry.name, formats: labels.join(','), ops: warmOps, rounds: String(rounds), minMs: args['min-ms'] })
      const done = await open(query.toString())
      if (!gcNoted) {
        await appendFile(file, `gc: ${done.gc ? 'forced before each sample' : 'not available'}\n`)
        gcNoted = true
      }
      done.results.forEach(add)
      for (const [format, path] of Object.entries(done.paths ?? {})) {
        await appendFile(pathsFile, `${entry.name},${format},${path}\n`)
      }
    }
    if (ops.includes('cold')) {
      const available = formats.filter((f) => labels.includes(f.label) && entry.formats[f.name])
      for (let round = 0; round < rounds; round++) {
        for (const format of available) {
          const done = await open(new URLSearchParams({ case: entry.name, cold: format.label }).toString())
          done.results.forEach(add)
        }
      }
    }
    await appendFile(file, lines.join('\n') + '\n')
    console.error(`  ${entry.name}: ${lines.length} samples`)
  }
  await browser.close()
  await browserServer.close()
}
server.close()
console.error(`results in ${args.out}`)
