/** `{param}` placeholders in saved query filter DSL values */

const PARAM_RE = /\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g

export function extractFilterParams(filter: Record<string, unknown> | undefined): string[] {
  if (!filter) return []
  const seen = new Set<string>()
  collectParams(filter, seen)
  return [...seen].sort()
}

function collectParams(node: Record<string, unknown>, seen: Set<string>) {
  const t = String(node.type ?? '').toUpperCase()
  const val = node.val
  if (t === 'AND' || t === 'OR') {
    if (Array.isArray(val)) {
      for (const item of val) {
        if (item && typeof item === 'object' && !Array.isArray(item)) {
          collectParams(item as Record<string, unknown>, seen)
        }
      }
    }
    return
  }
  recordParamNames(val, seen)
}

function recordParamNames(v: unknown, seen: Set<string>) {
  if (typeof v === 'string') {
    for (const m of v.matchAll(PARAM_RE)) {
      if (m[1]) seen.add(m[1])
    }
    return
  }
  if (Array.isArray(v)) {
    for (const item of v) recordParamNames(item, seen)
  }
}
