import { test, expect } from '@playwright/test'
import { addColumn, apiRequest, createTable, jsonBody, uniqueName } from '../lib/api'

test.describe('DataSource API', () => {
  test('CRUD and query with merged filter', async ({ request }) => {
    const table = uniqueName('ds_tbl')
    const dsName = uniqueName('ds_view')
    const tableQ = `table_id=${encodeURIComponent(table)}`

    await createTable(request, table)
    const title = await addColumn(request, table, 'title', 'text', 1)
    const score = await addColumn(request, table, 'score', 'double', 2)
    const active = await addColumn(request, table, 'active', 'bool', 3)

    let res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(table)}/rows`, {
      cells: {
        [title.column.id]: { stringValue: 'Alpha' },
        [score.column.id]: { numberValue: 10 },
        [active.column.id]: { boolValue: true },
      },
    })
    expect(res.status()).toBe(200)

    res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(table)}/rows`, {
      cells: {
        [title.column.id]: { stringValue: 'Beta' },
        [score.column.id]: { numberValue: 20 },
        [active.column.id]: { boolValue: false },
      },
    })
    expect(res.status()).toBe(200)

    res = await apiRequest(request, 'POST', '/v1/data-sources', {
      name: dsName,
      label: 'Active only',
      tableId: table,
      columnIds: [title.column.name, score.column.name],
      filter: { type: 'EQ', attr: active.column.name, val: true },
      sort: [{ attribute: score.column.name, sortOrder: 'DESC' }],
    })
    expect(res.status()).toBe(200)
    const created = await jsonBody<{ dataSource: { id: string; name: string } }>(res)
    expect(created.dataSource.id).toBe(dsName)

    res = await apiRequest(request, 'GET', `/v1/data-sources/${encodeURIComponent(dsName)}?${tableQ}`)
    expect(res.status()).toBe(200)

    res = await apiRequest(request, 'GET', `/v1/data-sources?${tableQ}`)
    const listed = await jsonBody<{ dataSources?: { id: string }[] }>(res)
    expect(listed.dataSources?.some((d) => d.id === dsName)).toBe(true)

    res = await apiRequest(request, 'POST', `/v1/data-sources/${encodeURIComponent(dsName)}:query?${tableQ}`, {
      pageSize: 10,
    })
    expect(res.status()).toBe(200)
    const q1 = await jsonBody<{ rows?: { cells?: Record<string, unknown> }[] }>(res)
    expect(q1.rows?.length).toBe(1)

    res = await apiRequest(request, 'POST', `/v1/data-sources/${encodeURIComponent(dsName)}:query?${tableQ}`, {
      pageSize: 10,
      filter: { type: 'EQ', attr: title.column.name, val: 'Alpha' },
    })
    expect(res.status()).toBe(200)
    const q2 = await jsonBody<{ rows?: unknown[] }>(res)
    expect(q2.rows?.length).toBe(1)

    res = await apiRequest(request, 'PATCH', `/v1/data-sources/${encodeURIComponent(dsName)}?${tableQ}`, {
      label: 'Updated label',
    })
    expect(res.status()).toBe(200)

    res = await apiRequest(request, 'DELETE', `/v1/data-sources/${encodeURIComponent(dsName)}?${tableQ}`)
    expect(res.status()).toBe(200)

    res = await apiRequest(request, 'GET', `/v1/data-sources/${encodeURIComponent(dsName)}?${tableQ}`)
    expect(res.status()).toBe(404)
  })
})
