const defaultBase = () =>
  (import.meta.env.VITE_API_BASE as string | undefined) || 'http://localhost:8080'

export type ApiOpts = {
  baseUrl?: string
  /** Data API. Defaults to VITE_DATA_API_BASE or baseUrl (same origin as admin). */
  dataUrl?: string
  /** Tenant id (X-Tenant-Id); required. */
  tenantId?: string
  /** Base id within tenant (X-Base-Id); defaults server-side to first base. */
  baseId?: string
}

function headers(opts: ApiOpts = {}): HeadersInit {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (opts.tenantId) h['X-Tenant-Id'] = opts.tenantId
  if (opts.baseId) h['X-Base-Id'] = opts.baseId
  return h
}

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text()
  if (!res.ok) {
    throw new Error(text || res.statusText || `HTTP ${res.status}`)
  }
  return text ? (JSON.parse(text) as T) : ({} as T)
}

function base(opts: ApiOpts) {
  return opts.baseUrl ?? defaultBase()
}

function dataBase(opts: ApiOpts) {
  return (
    opts.dataUrl ||
    (import.meta.env.VITE_DATA_API_BASE as string | undefined) ||
    base(opts)
  )
}

// -------- Tables --------

export async function listTables(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables`, { headers: headers(opts) })
  return parseJson<{ tables: Table[] }>(res)
}

export async function createTable(
  name: string,
  opts: ApiOpts = {},
  body: { schemaName?: string; idType?: string; label?: string } = {},
) {
  const res = await fetch(`${base(opts)}/v1/tables`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify({ name, ...body }),
  })
  return parseJson<{ table: Table }>(res)
}

export async function deleteTable(id: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: headers(opts),
  })
  return parseJson<Record<string, unknown>>(res)
}

export async function getTableSchema(tableId: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/schema`, {
    headers: headers(opts),
  })
  return parseJson<{ table: Table; columns: Column[]; indexes: Index[] }>(res)
}

export async function renameTable(id: string, newName: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(id)}:rename`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify({ id, newName }),
  })
  return parseJson<{ table: Table }>(res)
}

// -------- Columns (GET/POST /v1/columns, PATCH/DELETE /v1/columns/{id}) --------

export async function listColumns(tableId: string, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${base(opts)}/v1/columns?${q}`, {
    headers: headers(opts),
  })
  return parseJson<{ columns: Column[] }>(res)
}

export async function createColumn(
  body: {
    tableId: string
    name: string
    label?: string
    typeId: string
    isNullable?: boolean
    position?: number
    config?: Record<string, unknown>
  },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/columns`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify(body),
  })
  return parseJson<{ column: Column }>(res)
}

export async function updateColumn(
  tableId: string,
  columnRef: string,
  body: {
    name?: string
    label?: string
    typeId?: string
    isNullable?: boolean
    position?: number
    config?: Record<string, unknown>
  },
  opts: ApiOpts = {},
) {
  const q = tableId ? `?table_id=${encodeURIComponent(tableId)}` : ''
  const res = await fetch(`${base(opts)}/v1/columns/${encodeURIComponent(columnRef)}${q}`, {
    method: 'PATCH',
    headers: headers(opts),
    body: JSON.stringify({ ...body, tableId }),
  })
  return parseJson<{ column: Column }>(res)
}

export async function deleteColumn(tableId: string, columnRef: string, opts: ApiOpts = {}) {
  const q = tableId ? `?table_id=${encodeURIComponent(tableId)}` : ''
  const res = await fetch(`${base(opts)}/v1/columns/${encodeURIComponent(columnRef)}${q}`, {
    method: 'DELETE',
    headers: headers(opts),
  })
  return parseJson<Record<string, unknown>>(res)
}

// -------- Indexes (today: PG catalog via /v1/indexes; RFC: JSONB partial / FTS on virtual_records) --------

export async function listIndexes(tableId: string, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${base(opts)}/v1/indexes?${q}`, {
    headers: headers(opts),
  })
  return parseJson<{ indexes: Index[] }>(res)
}

export async function createIndex(
  body: { tableId: string; name: string; columnIds: string[]; isUnique?: boolean },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/indexes`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify(body),
  })
  return parseJson<{ index: Index }>(res)
}

export async function deleteIndex(pgIndexName: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/indexes/${encodeURIComponent(pgIndexName)}`, {
    method: 'DELETE',
    headers: headers(opts),
  })
  return parseJson<Record<string, unknown>>(res)
}

// -------- Saved queries --------

export type SortOrder = {
  attribute?: string
  sortOrder?: string
}

export type Query = {
  id: string
  name?: string
  label?: string
  tableId?: string
  filter?: Record<string, unknown>
  sort?: SortOrder[]
  columnIds?: string[]
  config?: Record<string, unknown>
}

export async function listQueries(tableId: string | undefined, opts: ApiOpts = {}) {
  const q = new URLSearchParams()
  if (tableId) q.set('table_id', tableId)
  const suffix = q.toString() ? `?${q}` : ''
  const res = await fetch(`${base(opts)}/v1/admin/queries${suffix}`, {
    headers: headers(opts),
  })
  return parseJson<{ queries: Query[] }>(res)
}

export async function createQuery(
  body: {
    name: string
    label?: string
    tableId: string
    filter?: Record<string, unknown>
    sort?: SortOrder[]
    columnIds?: string[]
  },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/admin/queries`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify(body),
  })
  return parseJson<{ query: Query }>(res)
}

export async function deleteQuery(tableId: string, name: string, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${base(opts)}/v1/admin/queries/${encodeURIComponent(name)}?${q}`, {
    method: 'DELETE',
    headers: headers(opts),
  })
  return parseJson<Record<string, unknown>>(res)
}

export async function executeQuery(
  tableId: string,
  name: string,
  body: {
    pageSize?: number
    filter?: Record<string, unknown>
    /** Replaces `{param}` placeholders in the saved query's stored filter */
    params?: Record<string, unknown>
    /** Intersected with query column_names; omit to use query projection */
    columnIds?: string[]
  } = {},
  opts: ApiOpts = {},
) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${dataBase(opts)}/v1/data/queries/${encodeURIComponent(name)}?${q}`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify(body),
  })
  return parseJson<{ rows: Row[]; nextPageToken?: string; count?: number }>(res)
}

// -------- Types (built-in, read-only) --------

export async function listTypes(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/types`, { headers: headers(opts) })
  return parseJson<{ types: ColType[] }>(res)
}

// -------- Rows --------

export async function listRows(tableId: string, pageSize: number, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ pageSize: String(pageSize) })
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows?${q}`,
    { headers: headers(opts) },
  )
  return parseJson<{ rows: Row[]; nextPageToken?: string }>(res)
}

export async function queryRows(
  tableId: string,
  body: {
    pageSize?: number
    pageToken?: string
    filter?: Record<string, unknown>
    sort?: SortOrder[]
    columnIds?: string[]
  } = {},
  opts: ApiOpts = {},
) {
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows:query`,
    {
      method: 'POST',
      headers: headers(opts),
      body: JSON.stringify({ tableId, ...body }),
    },
  )
  return parseJson<{ rows: Row[]; nextPageToken?: string; count?: number }>(res)
}

export async function createRow(
  tableId: string,
  fields: Record<string, unknown>,
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows`, {
    method: 'POST',
    headers: headers(opts),
    body: JSON.stringify({ tableId, ...fields }),
  })
  return parseJson<{ row: Row }>(res)
}

export async function updateRow(
  tableId: string,
  rowId: string,
  fields: Record<string, unknown>,
  opts: ApiOpts = {},
) {
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows/${encodeURIComponent(rowId)}`,
    {
      method: 'PATCH',
      headers: headers(opts),
      body: JSON.stringify(fields),
    },
  )
  return parseJson<{ row: Row }>(res)
}

export async function bulkDeleteRows(tableId: string, rowIds: string[], opts: ApiOpts = {}) {
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows:bulkDelete`,
    {
      method: 'POST',
      headers: headers(opts),
      body: JSON.stringify({ tableId, rowIds }),
    },
  )
  return parseJson<Record<string, unknown>>(res)
}

export async function importRows(
  tableId: string,
  rows: Record<string, unknown>[],
  opts: ApiOpts = {},
) {
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows:import`,
    {
      method: 'POST',
      headers: headers(opts),
      body: JSON.stringify({ tableId, format: 1, rows }),
    },
  )
  return parseJson<{ rows: Row[]; insertedCount: number }>(res)
}

// -------- Platform --------

export async function createTenant(
  body: {
    id: string
    displayName?: string
    dataDsn?: string
    createDatabase?: boolean
    recordStore?: 'shared' | 'dedicated'
  },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/admin/tenants`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return parseJson<{ id: string }>(res)
}

export async function getDatabaseConnection(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/database/connection`, {
    headers: headers(opts),
  })
  return parseJson<ConnectionInfo>(res)
}

export type Tenant = {
  tenantId: string
  name?: string
  status?: string
  recordStore?: string
}

export async function listTenants(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/admin/tenants`, {
    headers: headers(opts),
  })
  return parseJson<{ tenants: Tenant[] }>(res)
}

// -------- Types --------

export type Table = {
  id?: string
  name?: string
  label?: string
  schemaName?: string
  idType?: 'uuid' | 'int8' | string
}

export type Column = {
  id: string
  name: string
  label?: string
  typeId: string
  /** Effective value type for filters (from config.result_type_id). */
  resultTypeId?: string
  isNullable?: boolean
  position?: number
  config?: Record<string, unknown>
}

export type Index = {
  id: string
  tableId?: string
  name?: string
  pgIndex?: string
  columnIds?: string[]
  isUnique?: boolean
}

export type ColType = {
  id: string
  name?: string
  pgType?: string
  config?: Record<string, unknown>
}

export type Row = { id: string } & Record<string, unknown>

export type CellValue = {
  stringValue?: string
  numberValue?: number
  boolValue?: boolean
  timestampValue?: string
  jsonValue?: unknown
}

export type ConnectionInfo = {
  host: string
  port: number
  database: string
  user: string
  urlWithoutPassword: string
  psqlCommand: string
  passwordSourceHint: string
}

export function formatCell(v: unknown): string {
  if (v === undefined || v === null) return ''
  if (Array.isArray(v)) {
    return v
      .map((x) => (x === null || x === undefined ? '' : String(x)))
      .filter(Boolean)
      .join(', ')
  }
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (typeof v !== 'object') return String(v)
  const o = v as CellValue
  if (o.stringValue !== undefined) return o.stringValue
  if (o.numberValue !== undefined) return String(o.numberValue)
  if (o.boolValue !== undefined) return String(o.boolValue)
  if (o.timestampValue) return o.timestampValue
  if (o.jsonValue !== undefined) {
    if (Array.isArray(o.jsonValue)) {
      return o.jsonValue.map((x) => String(x)).join(', ')
    }
    try {
      return JSON.stringify(o.jsonValue)
    } catch {
      return '[json]'
    }
  }
  return JSON.stringify(v)
}

function coerceArrayElements(items: unknown[], typeId: string): unknown[] {
  const base = typeId.replace(/_array$/, '')
  return items.map((item) => {
    const s = String(item).trim()
    switch (base) {
      case 'bool':
      case 'boolean':
        return s === 'true' || s === '1'
      case 'int8':
      case 'integer':
      case 'double':
      case 'number':
      case 'bigint':
        return Number(s)
      default:
        return s
    }
  })
}

function parseArrayInput(raw: string, typeId: string): unknown[] | undefined {
  const s = raw.trim()
  if (!s) return undefined
  if (s.startsWith('[')) {
    try {
      const parsed = JSON.parse(s) as unknown
      if (!Array.isArray(parsed)) return undefined
      return coerceArrayElements(parsed, typeId)
    } catch {
      return undefined
    }
  }
  const items = s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
  if (!items.length) return undefined
  return coerceArrayElements(items, typeId)
}

export function cellToNative(typeId: string, raw: string, allowEmpty = false): unknown {
  const cell = cellFromString(typeId, raw, allowEmpty)
  if (!cell) return undefined
  if (cell.stringValue !== undefined) return cell.stringValue
  if (cell.numberValue !== undefined) return cell.numberValue
  if (cell.boolValue !== undefined) return cell.boolValue
  if (cell.timestampValue !== undefined) return cell.timestampValue
  if (cell.jsonValue !== undefined) return cell.jsonValue
  return undefined
}

export function cellFromString(typeId: string, raw: string, allowEmpty = false): CellValue | undefined {
  const s = raw.trim()
  if (s === '') {
    if (!allowEmpty) return undefined
    if (typeId.endsWith('_array')) return { jsonValue: [] }
    switch (typeId) {
      case 'number':
      case 'double':
      case 'precision':
      case 'integer':
      case 'int8':
        return { stringValue: '' }
      case 'bool':
        return { stringValue: '' }
      default:
        return { stringValue: '' }
    }
  }
  if (typeId.endsWith('_array')) {
    const arr = parseArrayInput(s, typeId)
    if (!arr) return undefined
    return { jsonValue: arr }
  }
  switch (typeId) {
    case 'number':
    case 'double':
    case 'precision':
    case 'bigint':
      return { numberValue: Number(s) }
    case 'bool':
    case 'boolean':
      return { boolValue: s === 'true' || s === '1' }
    case 'integer':
    case 'int8':
      return { numberValue: Number(s) }
    case 'datetime':
    case 'timestamptz':
    case 'timestamp':
    case 'date':
      return { timestampValue: s }
    default:
      return { stringValue: s }
  }
}
