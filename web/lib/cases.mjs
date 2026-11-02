// Case selection, the same patterns colbinbench takes: comma-separated case
// names or globs where `*` matches within one path segment, or "all".

export function selectCases(cases, patterns = 'all') {
  const tests = patterns.split(',').map((p) => p.trim()).filter(Boolean).map((pattern) => {
    if (pattern === 'all') return () => true
    const source = pattern.split('*').map((part) => part.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('[^/]*')
    const regex = new RegExp(`^${source}$`)
    return (name) => regex.test(name)
  })
  return cases.filter((c) => tests.some((test) => test(c.name)))
}
