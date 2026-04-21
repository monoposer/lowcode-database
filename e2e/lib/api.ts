import type { APIRequestContext, APIResponse } from '@playwright/test'

export const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080'
export const TENANT_ID = process.env.E2E_TENANT_ID || 'default'

export function tenantHeaders(extra?: Record<string, string>): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Tenant-Id': TENANT_ID,
    ...extra,
  }
}

export function uniqueName(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

export async function apiRequest(
  request: APIRequestContext,
  method: string,
  path: string,
  body?: unknown,
): Promise<APIResponse> {
  const url = `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`
  const opts = { headers: tenantHeaders() }
  switch (method.toUpperCase()) {
    case 'GET':
      return request.get(url, opts)
    case 'POST':
      return request.post(url, { ...opts, data: body })
    case 'PATCH':
      return request.patch(url, { ...opts, data: body })
    case 'DELETE':
      return request.delete(url, opts)
    default:
      throw new Error(`unsupported method ${method}`)
  }
}

export async function jsonBody<T>(res: APIResponse): Promise<T> {
  const text = await res.text()
  if (!text) return {} as T
  return JSON.parse(text) as T
}

export type Column = { id: string; name: string; typeId?: string; config?: Record<string, unknown> }
export type Row = { id: string; cells?: Record<string, unknown> }

export async function createTable(request: APIRequestContext, name: string) {
  const res = await apiRequest(request, 'POST', '/v1/tables', { name })
  if (res.status() !== 200) {
    throw new Error(`createTable ${name}: ${res.status()} ${await res.text()}`)
  }
  return jsonBody<{ table: { id: string; name: string } }>(res)
}

export async function deleteTable(request: APIRequestContext, tableId: string) {
  await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(tableId)}`)
}

export async function addColumn(
  request: APIRequestContext,
  tableId: string,
  name: string,
  typeId: string,
  position: number,
  config?: Record<string, unknown>,
) {
  const res = await apiRequest(request, 'POST', '/v1/columns', {
    tableId,
    name,
    typeId,
    position,
    ...(config ? { config } : {}),
  })
  if (res.status() !== 200) {
    throw new Error(`addColumn ${name}: ${res.status()} ${await res.text()}`)
  }
  return jsonBody<{ column: Column }>(res)
}

export async function createRow(
  request: APIRequestContext,
  tableId: string,
  cells: Record<string, unknown>,
) {
  const res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(tableId)}/rows`, {
    cells,
  })
  if (res.status() !== 200) {
    throw new Error(`createRow: ${res.status()} ${await res.text()}`)
  }
  return jsonBody<{ row: Row }>(res)
}
