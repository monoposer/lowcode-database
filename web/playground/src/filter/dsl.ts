/** Filter DSL aligned with lowcode-database internal/dsl */

export type FilterOp =
  | 'EQ'
  | 'NEQ'
  | 'GT'
  | 'GTE'
  | 'LT'
  | 'LTE'
  | 'BETWEEN'
  | 'LIKE'
  | 'IN'
  | 'NIN'
  | 'EMPTY'
  | 'NOT_EMPTY'
  | 'ARRAY_HAS'
  | 'ARRAY_NOT_HAS'
  | 'ARRAY_OVERLAP'
  | 'ARRAY_NOT_OVERLAP'
  | 'ARRAY_CONTAINS'
  | 'ARRAY_NOT_CONTAINS'

export type FilterCondition = {
  id: string
  attr: string
  op: FilterOp
  /** Raw string from UI; coerced when building DSL */
  val: string
  /** Range end for BETWEEN */
  val2?: string
}

export type FilterGroup = {
  conjunction: 'AND' | 'OR'
  conditions: FilterCondition[]
}

export type FilterOpDef = {
  op: FilterOp
  label: string
  needsValue: boolean
  /** Comma-separated list for IN / NIN */
  listValue?: boolean
  /** Two bounds for BETWEEN */
  rangeValue?: boolean
}

const COMMON: FilterOpDef[] = [
  { op: 'EQ', label: 'is', needsValue: true },
  { op: 'NEQ', label: 'is not', needsValue: true },
  { op: 'EMPTY', label: 'is empty', needsValue: false },
  { op: 'NOT_EMPTY', label: 'is not empty', needsValue: false },
]

const TEXT_OPS: FilterOpDef[] = [
  ...COMMON,
  { op: 'LIKE', label: 'contains', needsValue: true },
  { op: 'IN', label: 'is any of', needsValue: true, listValue: true },
  { op: 'NIN', label: 'is none of', needsValue: true, listValue: true },
]

const COMPARABLE_OPS: FilterOpDef[] = [
  ...COMMON,
  { op: 'GT', label: '>', needsValue: true },
  { op: 'GTE', label: '≥', needsValue: true },
  { op: 'LT', label: '<', needsValue: true },
  { op: 'LTE', label: '≤', needsValue: true },
]

const DATETIME_OPS: FilterOpDef[] = [
  ...COMPARABLE_OPS,
  { op: 'BETWEEN', label: 'between', needsValue: true, rangeValue: true },
]

const BOOL_OPS: FilterOpDef[] = [{ op: 'EQ', label: 'is', needsValue: true }]

const LINK_OPS: FilterOpDef[] = [
  { op: 'ARRAY_HAS', label: 'contains', needsValue: true },
  { op: 'ARRAY_NOT_HAS', label: 'does not contain', needsValue: true },
  { op: 'ARRAY_OVERLAP', label: 'contains any of', needsValue: true, listValue: true },
  { op: 'EMPTY', label: 'is empty', needsValue: false },
  { op: 'NOT_EMPTY', label: 'is not empty', needsValue: false },
]

const ARRAY_OPS: FilterOpDef[] = [
  { op: 'ARRAY_HAS', label: 'has', needsValue: true },
  { op: 'ARRAY_NOT_HAS', label: 'does not have', needsValue: true },
  { op: 'ARRAY_OVERLAP', label: 'overlaps', needsValue: true, listValue: true },
  { op: 'ARRAY_NOT_OVERLAP', label: 'does not overlap', needsValue: true, listValue: true },
  { op: 'ARRAY_CONTAINS', label: 'contains all', needsValue: true, listValue: true },
  { op: 'ARRAY_NOT_CONTAINS', label: 'does not contain all', needsValue: true, listValue: true },
  { op: 'EMPTY', label: 'is empty', needsValue: false },
  { op: 'NOT_EMPTY', label: 'is not empty', needsValue: false },
]

const NUMERIC_TYPE_IDS = new Set(['number', 'rollup'])

/** Heuristic aligned with InferFormulaResultTypeId. */
export function inferFormulaResultTypeId(expression: string): 'number' | 'text' | 'datetime' | 'boolean' {
  const e = expression.trim()
  if (!e) return 'number'
  const u = e.toUpperCase()
  const has = (name: string) => new RegExp(`(?:^|[^A-Z0-9_])${name}\\(`).test(u)
  if (has('ISBLANK')) return 'boolean'
  if (has('DATEDIF') || has('YEAR') || has('MONTH') || has('DAY') || has('LEN') || has('MIN') || has('MAX') || has('INT') || has('WEEKDAY')) {
    return 'number'
  }
  if (has('CONCAT') || has('TEXT') || has('LOWER') || has('UPPER') || has('TRIM')) return 'text'
  if (has('DATEVALUE') || has('DATE') || has('TODAY') || has('NOW')) return 'datetime'
  if (has('MID')) return 'text'
  if (/["']/.test(e)) return 'text'
  return 'number'
}

/** Heuristic: text-returning formulas (CONCAT, quoted literals) vs numeric. */
export function formulaExpressionLooksTextual(expression: string): boolean {
  return inferFormulaResultTypeId(expression) === 'text'
}

/** How the value field should render for the current condition. */
export function filterValueInputKind(
  filterType: string,
  op: FilterOp,
  listValue?: boolean,
  rawTypeId?: string,
): 'bool' | 'number' | 'datetime' | 'text' {
  if (filterType === 'boolean') return 'bool'
  if (op === 'LIKE' || op === 'ARRAY_HAS' || listValue) return 'text'
  if (filterType === 'datetime') return 'datetime'
  if (op === 'GT' || op === 'GTE' || op === 'LT' || op === 'LTE') return 'number'
  if (filterType === 'number') return 'number'
  return 'text'
}

const DATETIME_TYPE_IDS = new Set(['datetime'])

/** Prefer API resultTypeId; fall back to typeId / formula expression heuristic. */
export function effectiveFilterTypeId(
  typeId: string,
  expression?: string,
  resultTypeId?: string,
): string {
  const rt = resultTypeId?.trim()
  if (rt) {
    if (NUMERIC_TYPE_IDS.has(rt)) return 'number'
    if (DATETIME_TYPE_IDS.has(rt)) return 'datetime'
    return rt
  }
  if (typeId === 'formula') {
    return inferFormulaResultTypeId(expression ?? '')
  }
  if (NUMERIC_TYPE_IDS.has(typeId)) return 'number'
  if (DATETIME_TYPE_IDS.has(typeId)) return 'datetime'
  if (typeId === 'lookup' || typeId === 'rollup') return 'number'
  return typeId
}

export function isNumericFilterType(typeId: string): boolean {
  return effectiveFilterTypeId(typeId) === 'number'
}

export function isDateTimeFilterType(typeId: string): boolean {
  return DATETIME_TYPE_IDS.has(typeId)
}

export function dateTimeInputType(typeId: string): 'date' | 'datetime-local' {
  return 'datetime-local'
}

export function filterOpsForColumn(col: {
  typeId: string
  resultTypeId?: string
  expression?: string
  isArray?: boolean
}): FilterOpDef[] {
  return filterOpsForType(col.typeId, col.expression, col.resultTypeId, col.isArray)
}

export function filterOpsForType(
  typeId: string,
  expression?: string,
  resultTypeId?: string,
  isArray?: boolean,
): FilterOpDef[] {
  if (typeId === 'link') return LINK_OPS
  if (isArray) return ARRAY_OPS
  const t = effectiveFilterTypeId(typeId, expression, resultTypeId)
  if (t === 'boolean') return BOOL_OPS
  if (t === 'number') return COMPARABLE_OPS
  if (DATETIME_TYPE_IDS.has(t)) return DATETIME_OPS
  return TEXT_OPS
}

export function newFilterCondition(attr = '', isArray = false, typeId?: string): FilterCondition {
  const op = isArray || typeId === 'link' ? 'ARRAY_HAS' : 'EQ'
  return { id: crypto.randomUUID(), attr, op, val: '' }
}

export function emptyFilterGroup(): FilterGroup {
  return { conjunction: 'AND', conditions: [] }
}

function coerceScalar(
  val: string,
  typeId: string,
  expression?: string,
  resultTypeId?: string,
): unknown {
  const s = val.trim()
  const t = effectiveFilterTypeId(typeId, expression, resultTypeId)
  if (t === 'boolean') return s === 'true' || s === '1'
  if (t === 'number') {
    const n = Number(s)
    return Number.isNaN(n) ? s : n
  }
  return s
}

function parseList(
  val: string,
  typeId: string,
  expression?: string,
  resultTypeId?: string,
): unknown[] {
  return val
    .split(',')
    .map((p) => p.trim())
    .filter((p) => p.length > 0)
    .map((p) => coerceScalar(p, typeId, expression, resultTypeId))
}

/** Build API filter object; returns undefined when no valid conditions. */
export function buildFilterDSL(
  group: FilterGroup,
  columnTypes: Record<string, string>,
  columnExpressions?: Record<string, string>,
  columnResultTypes?: Record<string, string>,
  columnIsArray?: Record<string, boolean>,
): Record<string, unknown> | undefined {
  const nodes: Record<string, unknown>[] = []
  for (const c of group.conditions) {
    if (!c.attr) continue
    const typeId = columnTypes[c.attr] ?? 'text'
    const resultTypeId = columnResultTypes?.[c.attr]
    const isArray = columnIsArray?.[c.attr] === true
    const def = filterOpsForType(typeId, columnExpressions?.[c.attr], resultTypeId, isArray).find(
      (d) => d.op === c.op,
    )
    if (!def) continue

    if (!def.needsValue) {
      nodes.push({ type: c.op, attr: c.attr })
      continue
    }

    const expr = columnExpressions?.[c.attr]
    if (def.rangeValue) {
      if (!c.val.trim() || !c.val2?.trim()) continue
      nodes.push({
        type: c.op,
        attr: c.attr,
        val: [
          coerceScalar(c.val, typeId, expr, resultTypeId),
          coerceScalar(c.val2, typeId, expr, resultTypeId),
        ],
      })
      continue
    }
    if (!c.val.trim()) continue
    if (def.listValue || c.op === 'ARRAY_OVERLAP' || c.op === 'ARRAY_NOT_OVERLAP' || c.op === 'ARRAY_CONTAINS' || c.op === 'ARRAY_NOT_CONTAINS') {
      nodes.push({ type: c.op, attr: c.attr, val: parseList(c.val, typeId, expr, resultTypeId) })
    } else {
      nodes.push({
        type: c.op,
        attr: c.attr,
        val: coerceScalar(c.val, typeId, expr, resultTypeId),
      })
    }
  }
  if (nodes.length === 0) return undefined
  if (nodes.length === 1) return nodes[0]
  return { type: group.conjunction, val: nodes }
}
