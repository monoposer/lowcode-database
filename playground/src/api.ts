const defaultBase = () =>
  (import.meta.env.VITE_API_BASE as string | undefined) || 'http://localhost:8080'

export type ApiOpts = {
  baseUrl?: string
  tenantId?: string
}

function headers(tenantId?: string): HeadersInit {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (tenantId) h['X-Tenant-Id'] = tenantId
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

// -------- Tables --------

export async function listTables(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables`, { headers: headers(opts.tenantId) })
  return parseJson<{ tables: Table[] }>(res)
}

export async function createTable(name: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables`, {
    method: 'POST',
    headers: headers(opts.tenantId),
    body: JSON.stringify({ name }),
  })
  return parseJson<{ table: Table }>(res)
}

export async function deleteTable(id: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: headers(opts.tenantId),
  })
  return parseJson<Record<string, unknown>>(res)
}

export async function getTableSchema(tableId: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/schema`, {
    headers: headers(opts.tenantId),
  })
  return parseJson<{ table: Table; columns: Column[]; indexes: Index[] }>(res)
}

export async function renameTable(id: string, newName: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(id)}:rename`, {
    method: 'POST',
    headers: headers(opts.tenantId),
    body: JSON.stringify({ id, newName }),
  })
  return parseJson<{ table: Table }>(res)
}

// -------- Columns (GET/POST /v1/columns, PATCH/DELETE /v1/columns/{id}) --------

export async function listColumns(tableId: string, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${base(opts)}/v1/columns?${q}`, {
    headers: headers(opts.tenantId),
  })
  return parseJson<{ columns: Column[] }>(res)
}

export async function createColumn(
  body: {
    tableId: string
    name: string
    typeId: string
    isNullable?: boolean
    position?: number
    config?: Record<string, unknown>
  },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/columns`, {
    method: 'POST',
    headers: headers(opts.tenantId),
    body: JSON.stringify(body),
  })
  return parseJson<{ column: Column }>(res)
}

export async function updateColumn(
  id: string,
  body: {
    name?: string
    isNullable?: boolean
    position?: number
    config?: Record<string, unknown>
  },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/columns/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: headers(opts.tenantId),
    body: JSON.stringify(body),
  })
  return parseJson<{ column: Column }>(res)
}

export async function deleteColumn(id: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/columns/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: headers(opts.tenantId),
  })
  return parseJson<Record<string, unknown>>(res)
}

// -------- Indexes (PG catalog via /v1/indexes) --------

export async function listIndexes(tableId: string, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ table_id: tableId })
  const res = await fetch(`${base(opts)}/v1/indexes?${q}`, {
    headers: headers(opts.tenantId),
  })
  return parseJson<{ indexes: Index[] }>(res)
}

export async function createIndex(
  body: { tableId: string; name: string; columnIds: string[]; isUnique?: boolean },
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/indexes`, {
    method: 'POST',
    headers: headers(opts.tenantId),
    body: JSON.stringify(body),
  })
  return parseJson<{ index: Index }>(res)
}

export async function deleteIndex(pgIndexName: string, opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/indexes/${encodeURIComponent(pgIndexName)}`, {
    method: 'DELETE',
    headers: headers(opts.tenantId),
  })
  return parseJson<Record<string, unknown>>(res)
}

// -------- Types (built-in, read-only) --------

export async function listTypes(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/types`, { headers: headers(opts.tenantId) })
  return parseJson<{ types: ColType[] }>(res)
}

// -------- Rows --------

export async function listRows(tableId: string, pageSize: number, opts: ApiOpts = {}) {
  const q = new URLSearchParams({ pageSize: String(pageSize) })
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows?${q}`,
    { headers: headers(opts.tenantId) },
  )
  return parseJson<{ rows: Row[]; nextPageToken?: string }>(res)
}

export async function createRow(
  tableId: string,
  cells: Record<string, CellValue>,
  opts: ApiOpts = {},
) {
  const res = await fetch(`${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows`, {
    method: 'POST',
    headers: headers(opts.tenantId),
    body: JSON.stringify({ tableId, cells }),
  })
  return parseJson<{ row: Row }>(res)
}

export async function bulkDeleteRows(tableId: string, rowIds: string[], opts: ApiOpts = {}) {
  const res = await fetch(
    `${base(opts)}/v1/tables/${encodeURIComponent(tableId)}/rows:bulkDelete`,
    {
      method: 'POST',
      headers: headers(opts.tenantId),
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
      headers: headers(opts.tenantId),
      body: JSON.stringify({ tableId, format: 1, rows }),
    },
  )
  return parseJson<{ rows: Row[]; insertedCount: number }>(res)
}

// -------- Platform --------

export async function getDatabaseConnection(opts: ApiOpts = {}) {
  const res = await fetch(`${base(opts)}/v1/database/connection`, {
    headers: headers(opts.tenantId),
  })
  return parseJson<ConnectionInfo>(res)
}

// -------- Types --------

export type Table = {
  id?: string
  name?: string
  schemaName?: string
  tableName?: string
}

export type Column = {
  id: string
  name: string
  typeId: string
  pgColumn?: string
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
}

export type Row = {
  id: string
  cells: Record<string, CellValue>
}

export type CellValue = {
  stringValue?: string
  numberValue?: number
  boolValue?: boolean
  timestampValue?: string
  jsonValue?: Record<string, unknown>
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
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (typeof v !== 'object') return String(v)
  const o = v as CellValue
  if (o.stringValue !== undefined) return o.stringValue
  if (o.numberValue !== undefined) return String(o.numberValue)
  if (o.boolValue !== undefined) return String(o.boolValue)
  if (o.timestampValue) return o.timestampValue
  if (o.jsonValue !== undefined) {
    try {
      return JSON.stringify(o.jsonValue)
    } catch {
      return '[json]'
    }
  }
  return JSON.stringify(v)
}

export function cellFromString(typeId: string, raw: string): CellValue | undefined {
  const s = raw.trim()
  if (s === '') return undefined
  switch (typeId) {
    case 'number':
    case 'double':
    case 'precision':
      return { numberValue: Number(s) }
    case 'bool':
      return { boolValue: s === 'true' || s === '1' }
    case 'integer':
    case 'int8':
      return { numberValue: Number(s) }
    default:
      return { stringValue: s }
  }
}
