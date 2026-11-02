// The common representation every format's decode is checked against: the
// Go JSON of the same case, parsed as described in web/README.md.

/** The reference value: the Go JSON as JSON.parse reads it. An integer past
 * 2^53 is the nearest Number here, as in every JSON reader; index.json's
 * unsafeInts says how many integer values of the case are past 2^53 (Go counts
 * them on the typed value), which is where a format that keeps them exact as
 * BigInt and the reference part ways by rounding only. */
export function parseReference(text) {
  return JSON.parse(text)
}

/**
 * Compares a decoded value with the reference. Returns null when they are the
 * same value, or a description of the first difference, with its path.
 *
 * Equal means:
 *   - numbers: the same Number; or the same once both are rounded to float32,
 *     for a float32 field, which Go's JSON writes as the shortest float32 text
 *     and a binary format carries as the float32 itself
 *   - a BigInt and a Number: the same once the BigInt is rounded to a Number,
 *     which is how the reference holds an integer past 2^53 (a column whose
 *     range passes 2^53 is BigInt throughout in colbin's fast path; protobuf-es
 *     reads every int64 as BigInt)
 *   - null and undefined: the same absence, and a key holding either is the
 *     same as the key not being there
 *   - objects: the same keys once `key` has mapped both sides' names to one
 *     spelling, for a format that renames fields (Arrow's snake_case,
 *     protobuf's camelCase): see canonicalKey
 */
export function compare(got, want, key = (k) => k, at = '$') {
  if (got === undefined) got = null
  if (want === undefined) want = null
  if (got === null || want === null) {
    return got === want ? null : `${at}: ${show(got)} against ${show(want)}`
  }
  if (typeof got === 'bigint' || typeof want === 'bigint') {
    if (typeof got === 'bigint' && typeof want === 'bigint' && got === want) return null
    const g = typeof got === 'bigint' ? Number(got) : got
    const w = typeof want === 'bigint' ? Number(want) : want
    if (typeof g === 'number' && g === w) return null
    return `${at}: ${show(got)} against ${show(want)}`
  }
  if (typeof got === 'number' && typeof want === 'number') {
    if (got === want || (Number.isNaN(got) && Number.isNaN(want))) return null
    if (Math.fround(got) === Math.fround(want)) return null
    return `${at}: ${got} against ${want}`
  }
  if (Array.isArray(want)) {
    if (!isList(got)) return `${at}: ${show(got)} against an array`
    if (got.length !== want.length) return `${at}: ${got.length} elements against ${want.length}`
    for (let i = 0; i < want.length; i++) {
      const diff = compare(got[i], want[i], key, `${at}[${i}]`)
      if (diff) return diff
    }
    return null
  }
  if (typeof want === 'object') {
    if (typeof got !== 'object' || Array.isArray(got)) return `${at}: ${show(got)} against an object`
    const gotKeys = new Map()
    for (const k of Object.keys(got)) {
      if (got[k] !== undefined && got[k] !== null && k !== '$typeName') gotKeys.set(key(k), k)
    }
    for (const k of Object.keys(want)) {
      const ck = key(k)
      const diff = compare(gotKeys.has(ck) ? got[gotKeys.get(ck)] : null, want[k], key, `${at}.${k}`)
      if (diff) return diff
      gotKeys.delete(ck)
    }
    if (gotKeys.size > 0) return `${at}: extra keys ${[...gotKeys.keys()].join(', ')}`
    return null
  }
  return got === want ? null : `${at}: ${show(got)} against ${show(want)}`
}

function isList(value) {
  return Array.isArray(value) || ArrayBuffer.isView(value)
}

function show(value) {
  if (typeof value === 'bigint') return `${value}n`
  if (typeof value === 'string') return JSON.stringify(value.length > 40 ? value.slice(0, 40) + '…' : value)
  if (Array.isArray(value)) return `an array of ${value.length}`
  if (value === null) return 'null'
  if (typeof value === 'object') return 'an object'
  return String(value)
}

/** One spelling for a field name across formats that rename it: lower case,
 * no underscores. "NationKey", "nation_key" and "nationKey" are the same key. */
export function canonicalKey(name) {
  return name.toLowerCase().replaceAll('_', '')
}
