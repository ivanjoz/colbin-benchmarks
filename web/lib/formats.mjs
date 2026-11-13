// Every format the browser benchmark decodes, behind one shape, shared by the
// Node verification (verify.mjs) and the page (bench/page.mjs).
//
// A format is:
//
//   name           the format's name in the Go matrix (index.json's keys)
//   label          the name in the benchmark lines
//   open(env)      once per page: compile a module, load generated code. env is
//                  { wasm: Uint8Array } (colbin's module bytes). Returns a handle.
//   prepare(h, f)  once per case: what a client holds per connection (colbin's
//                  schema section). f is the case's fixture files. Returns state.
//   decode(s, b)   bytes to objects ready to use: the operation measured
//   columns(s, b)  where the format has a column view: bytes to typed columns
//   access(s, b)   the access aggregate from bytes (the sum of one column)
//
// Nothing here is timed; the page times these functions as they are.

import { Codec } from 'colbin/asset'
// The plain-JavaScript decoder: no WebAssembly module in the page.
import { Decoder } from 'colbin/decode'
import { Unpackr } from 'msgpackr/unpack'
import { tableFromIPC, Vector, DataType } from 'apache-arrow'
import { canonicalKey } from './equal.mjs'
import { fromBinary, toJson } from '@bufbuild/protobuf'
import { ValueSchema } from '@bufbuild/protobuf/wkt'
import * as tpchPb from '../gen/tpch_pb.js'
import * as tpcdsPb from '../gen/tpcds_pb.js'
import * as publicbiPb from '../gen/publicbi_pb.js'
import * as schemastorePb from '../gen/schemastore_pb.js'

// The protobuf-es code buf generates from each dataset's .proto (npm run gen).
const protos = { tpch: tpchPb, tpcds: tpcdsPb, publicbi: publicbiPb, schemastore: schemastorePb }

const utf8 = new TextDecoder()

// MessagePack: tinylib/msgp writes a struct as a map keyed by field name, so
// it decodes to plain objects. An int64 is a Number when it fits 2^53 and a
// BigInt when it does not, the common representation (web/README.md).
// useRecords off: msgpackr's record extension is its own, not in the bytes.
const unpackr = new Unpackr({ mapsAsObjects: true, useRecords: false, int64AsType: 'auto' })

/** Arrow's framing in this repository (formats/arrow.go): one IPC file per
 * table, each behind its length as a little-endian uint32. */
function arrowTables(bytes) {
  const tables = []
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  for (let at = 0; at < bytes.length; ) {
    const length = view.getUint32(at, true)
    tables.push(tableFromIPC(bytes.subarray(at + 4, at + 4 + length)))
    at += 4 + length
  }
  return tables
}

/** An Arrow table as plain row objects: what a client holds after decode, the
 * same as every other format hands back. */
function arrowRows(table) {
  const rows = table.toArray()
  const nested = table.schema.fields.some((f) => DataType.isList(f.type) || DataType.isStruct(f.type))
  for (let i = 0; i < rows.length; i++) rows[i] = nested ? plain(rows[i]) : rows[i].toJSON()
  return rows
}

/** A nested Arrow value (a list's Vector, a struct's row) as plain arrays and
 * objects, all the way down. */
function plain(value) {
  if (value === null || typeof value !== 'object') return value
  if (value instanceof Vector) return Array.from(value, plain)
  if (ArrayBuffer.isView(value)) return Array.from(value)
  const out = {}
  for (const [key, field] of Object.entries(value.toJSON())) out[key] = plain(field)
  return out
}

/** Walks `path` (JSON keys) through a decoded value and sums the numbers at its
 * end: the same walk for every format that decodes to objects. */
export function sumPath(value, path) {
  let sum = 0
  // A format that renames fields (protobuf's camelCase) is walked by the key
  // that matches each path key's canonical spelling, found once per depth.
  const keys = []
  const keyAt = (node, depth) => {
    if (keys[depth] === undefined) {
      const want = path[depth]
      keys[depth] = want in node ? want : (Object.keys(node).find((k) => canonicalKey(k) === canonicalKey(want)) ?? want)
    }
    return keys[depth]
  }
  const walk = (node, depth) => {
    if (node === null || node === undefined) return
    if (Array.isArray(node)) {
      for (const item of node) walk(item, depth)
      return
    }
    if (depth === path.length) {
      sum += typeof node === 'bigint' ? Number(node) : node
      return
    }
    walk(node[keyAt(node, depth)], depth + 1)
  }
  walk(value, 0)
  return sum
}

/** colbin's column view of the access column: the table holding it, by the
 * path's first key when the root holds several, then the column by name. */
function columnSum(table, path) {
  const name = path[path.length - 1]
  const column = table.columns.find((c) => c.name === name)
  if (!column) return null
  let sum = 0
  const values = column.values
  if (column.valid) {
    // A nullable column: `valid` is a bitmap, one bit per row, set where the
    // row holds a value (the typed array holds 0 where it does not).
    const valid = column.valid
    for (let i = 0; i < values.length; i++) if ((valid[i >> 3] >> (i & 7)) & 1) sum += Number(values[i])
  } else if (column.kind === 'bigint') {
    for (let i = 0; i < values.length; i++) sum += Number(values[i])
  } else {
    for (let i = 0; i < values.length; i++) sum += values[i]
  }
  return sum
}

/** colbin's columns() result as one table for path: a bare table, or the table
 * under path[0] when the root holds several. */
function tableFor(result, path) {
  if (!result) return null
  if (Array.isArray(result.columns)) return result
  if (result.tables) return result.tables[path[0]] ?? null
  return result[path[0]] ?? null
}

export const formats = [
  {
    name: 'json-v2',
    label: 'json',
    open: async () => null,
    prepare: async () => ({}),
    // What a web client does with a JSON response body it holds as bytes.
    decode: (_, bytes) => JSON.parse(utf8.decode(bytes)),
    access: (state, bytes) => sumPath(JSON.parse(utf8.decode(bytes)), state.path),
  },
  {
    name: 'colbin',
    label: 'colbin',
    // One module per page, compiled from bytes: the network is not measured.
    open: async (env) => ({ module: await WebAssembly.compile(env.wasm) }),
    // The schema section travels once per connection; every message after it
    // is the body alone (colbin's recommended delivery).
    prepare: async (handle, files) => {
      const codec = Codec.fromModule(handle.module)
      codec.setSchema(files.schema)
      return { codec }
    },
    decode: (state, bytes) => state.codec.unmarshal(bytes),
    columns: (state, bytes) => state.codec.columns(bytes),
    access: (state, bytes) => {
      const table = tableFor(state.codec.columns(bytes), state.path)
      const sum = table && columnSum(table, state.path)
      return sum ?? sumPath(state.codec.unmarshal(bytes), state.path)
    },
    path: (state) => state.codec.lastPath,
  },
  {
    // colbin with its schema section in every message: a one-off response.
    name: 'colbin-json',
    label: 'colbin-standalone',
    open: async (env) => ({ module: await WebAssembly.compile(env.wasm) }),
    prepare: async (handle) => ({ codec: Codec.fromModule(handle.module) }),
    decode: (state, bytes) => state.codec.unmarshal(bytes),
    columns: (state, bytes) => state.codec.columns(bytes),
    access: (state, bytes) => {
      const table = tableFor(state.codec.columns(bytes), state.path)
      const sum = table && columnSum(table, state.path)
      return sum ?? sumPath(state.codec.unmarshal(bytes), state.path)
    },
    path: (state) => state.codec.lastPath,
  },
  {
    // colbin decoded in plain JavaScript (colbin/decode): no module to compile,
    // the same bytes and the same once-per-connection section as `colbin`.
    name: 'colbin',
    label: 'colbin-js',
    open: async () => null,
    prepare: async (_, files) => {
      const decoder = new Decoder()
      decoder.setSchema(files.schema)
      return { decoder }
    },
    decode: (state, bytes) => state.decoder.decode(bytes),
    columns: (state, bytes) => state.decoder.columns(bytes),
    access: (state, bytes) => {
      const table = tableFor(state.decoder.columns(bytes), state.path)
      const sum = table && columnSum(table, state.path)
      return sum ?? sumPath(state.decoder.decode(bytes), state.path)
    },
  },
  {
    name: 'colbin-json',
    label: 'colbin-js-standalone',
    open: async () => null,
    prepare: async () => ({ decoder: new Decoder() }),
    decode: (state, bytes) => state.decoder.decode(bytes),
    columns: (state, bytes) => state.decoder.columns(bytes),
    access: (state, bytes) => {
      const table = tableFor(state.decoder.columns(bytes), state.path)
      const sum = table && columnSum(table, state.path)
      return sum ?? sumPath(state.decoder.decode(bytes), state.path)
    },
  },
  {
    name: 'msgpack',
    label: 'msgpack',
    open: async () => null,
    prepare: async () => ({}),
    decode: (_, bytes) => unpackr.unpack(bytes),
    access: (state, bytes) => sumPath(unpackr.unpack(bytes), state.path),
  },
  {
    // protobuf-es: the message object the generated schema decodes to, as a
    // client of it uses it (int64 as BigInt, optional fields undefined when
    // absent). A dynamic case is google.protobuf.Value, turned into plain
    // JSON values with toJson, the only way to use one.
    name: 'protobuf',
    label: 'protobuf',
    key: canonicalKey,
    open: async () => null,
    prepare: async (_, files, entry) => ({
      schema: entry.dynamic ? ValueSchema : protos[entry.dataset][`${entry.type}Schema`],
      dynamic: entry.dynamic,
    }),
    decode: (state, bytes) => (state.dynamic ? toJson(ValueSchema, fromBinary(ValueSchema, bytes)) : fromBinary(state.schema, bytes)),
    access: (state, bytes) => sumPath(fromBinary(state.schema, bytes), state.path),
  },
  {
    // Arrow is columnar, the natural rival of colbin's columns(): columns is
    // the tables as Arrow hands them (vectors over the bytes), decode is the
    // tables as plain row objects, access sums the vector.
    name: 'arrow',
    label: 'arrow',
    key: canonicalKey,
    open: async () => null,
    prepare: async () => ({}),
    decode: (state, bytes) => {
      const tables = arrowTables(bytes)
      const root = {}
      state.tables.forEach((key, index) => { root[key] = arrowRows(tables[index]) })
      return root
    },
    columns: (_, bytes) => arrowTables(bytes),
    access: (state, bytes) => {
      const table = arrowTables(bytes)[state.tables.indexOf(state.path[0])]
      const want = canonicalKey(state.path[state.path.length - 1])
      const column = table.getChild(table.schema.fields.find((f) => canonicalKey(f.name) === want).name)
      let sum = 0
      for (const data of column.data) {
        const { values, nullBitmap, offset } = data
        for (let i = 0; i < data.length; i++) {
          const at = offset + i
          if (nullBitmap && data.nullCount > 0 && ((nullBitmap[at >> 3] >> (at & 7)) & 1) === 0) continue
          sum += Number(values[at])
        }
      }
      return sum
    },
  },
]

/** The cold start for a format, in a page that has not run it: the module's
 * compile and instantiation, then the first decode. */
export async function cold(format, env, files, bytes, entry) {
  const handle = await format.open(env)
  const state = await format.prepare(handle, files, entry)
  state.path = entry.access ?? []
  state.tables = entry.tables ?? []
  return format.decode(state, bytes)
}
