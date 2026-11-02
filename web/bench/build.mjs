// Bundles the benchmark page into bench/dist/page.js, an ES module with every
// decoder in it. run.mjs calls this before each run, so the page always holds
// the libraries web/package-lock.json pins.
import { build } from 'esbuild'
import { fileURLToPath } from 'node:url'

export async function bundle() {
  const outfile = fileURLToPath(new URL('./dist/page.js', import.meta.url))
  await build({
    entryPoints: [fileURLToPath(new URL('./page.mjs', import.meta.url))],
    bundle: true,
    format: 'esm',
    platform: 'browser',
    target: 'es2022',
    // Production builds of the libraries, minified as a site would ship them.
    minify: true,
    define: { 'process.env.NODE_ENV': '"production"' },
    outfile,
    logLevel: 'warning',
  })
  return outfile
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  console.log(await bundle())
}
