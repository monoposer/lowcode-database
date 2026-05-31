import { test, expect } from '@playwright/test'
import {
  addColumn,
  apiRequest,
  createRow,
  createTable,
  deleteTable,
  jsonBody,
  uniqueName,
} from '../lib/api'

/** Full HTTP workflow mirroring internal/service/integration_test.go */
test.describe('API full workflow', () => {
  test('vendor/order tables, relations, formula, data source, webhooks', async ({ request }) => {
    const vendorTable = uniqueName('e2e_vendor')
    const orderTable = uniqueName('e2e_order')
    const choiceName = uniqueName('e2e_status')

    await createTable(request, vendorTable)
    await createTable(request, orderTable)

    try {
      const nameCol = await addColumn(request, vendorTable, 'name', 'text', 1)
      const scoreCol = await addColumn(request, vendorTable, 'score', 'double', 2)
      const activeCol = await addColumn(request, vendorTable, 'active', 'bool', 3)
      const metaCol = await addColumn(request, vendorTable, 'meta', 'jsonb', 4)
      const vendorIDCol = await addColumn(request, vendorTable, 'legacy_id', 'int8', 5)
      const amountCol = await addColumn(request, orderTable, 'amount', 'precision', 1)
      const fkCol = await addColumn(request, orderTable, 'vendor_ref', 'relation_fk', 2, {
        target_table_id: vendorTable,
        target_column_id: vendorIDCol.column.id,
      })
      const linkCol = await addColumn(request, orderTable, 'vendor_id', 'uuid', 3)

      const relCol = await addColumn(request, vendorTable, 'orders', 'relationship', 6, {
        target_table_id: orderTable,
        link_column_id: linkCol.column.id,
        cardinality: 'many',
      })

      await addColumn(request, vendorTable, 'double_score', 'formula', 7, {
        expression: '{{score}} * 2',
      })

      let res = await apiRequest(request, 'POST', '/v1/choices', {
        name: choiceName,
        label: 'Status',
        values: [
          { value: 'active', label: 'Active' },
          { value: 'inactive', label: 'Inactive' },
        ],
      })
      expect(res.status()).toBe(200)

      await addColumn(request, vendorTable, 'status', choiceName, 8)

      res = await apiRequest(request, 'POST', '/v1/relations', {
        name: uniqueName('order_vendor'),
        kind: 'MANY_TO_ONE',
        sourceTableId: orderTable,
        sourceColumnId: fkCol.column.id,
        targetTableId: vendorTable,
        targetColumnId: vendorIDCol.column.id,
      })
      expect(res.status()).toBe(200)

      const vendorRow = await createRow(request, vendorTable, {
        [nameCol.column.id]: { stringValue: 'Acme' },
        [scoreCol.column.id]: { numberValue: 10 },
        [activeCol.column.id]: { boolValue: true },
        [metaCol.column.id]: { jsonValue: { tier: 'gold' } },
        [vendorIDCol.column.id]: { numberValue: 1001 },
      })

      await createRow(request, orderTable, {
        [amountCol.column.id]: { numberValue: 99.5 },
        [fkCol.column.id]: { numberValue: 1001 },
        [linkCol.column.id]: { stringValue: vendorRow.row.id },
      })

      res = await apiRequest(request, 'POST', '/v1/indexes', {
        tableId: vendorTable,
        name: 'idx_score',
        columnIds: [scoreCol.column.id],
      })
      expect(res.status()).toBe(200)

      res = await apiRequest(request, 'GET', `/v1/indexes?table_id=${encodeURIComponent(vendorTable)}`)
      expect(res.status()).toBe(200)
      const idxBody = await jsonBody<{ indexes?: unknown[] }>(res)
      expect((idxBody.indexes || []).length).toBeGreaterThan(0)

      res = await apiRequest(request, 'POST', '/v1/data-sources', {
        name: uniqueName('active_vendors'),
        label: 'Active Vendors',
        tableId: vendorTable,
        columnIds: [nameCol.column.name, scoreCol.column.name],
        filter: { type: 'EQ', attr: activeCol.column.name, val: true },
        sort: [{ attribute: scoreCol.column.name, sortOrder: 'DESC' }],
      })
      expect(res.status()).toBe(200)
      const ds = await jsonBody<{ dataSource: { id: string } }>(res)

      res = await apiRequest(
        request,
        'POST',
        `/v1/data-sources/${encodeURIComponent(ds.dataSource.id)}:query?table_id=${encodeURIComponent(vendorTable)}`,
        { pageSize: 10 },
      )
      expect(res.status()).toBe(200)
      const dsRows = await jsonBody<{ rows?: unknown[] }>(res)
      expect((dsRows.rows || []).length).toBeGreaterThan(0)

      res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(vendorTable)}/rows:query`, {
        filter: { type: 'EQ', attr: nameCol.column.id, val: 'Acme' },
        pageSize: 10,
      })
      expect(res.status()).toBe(200)
      const qrows = await jsonBody<{ rows?: unknown[] }>(res)
      expect(qrows.rows?.length).toBe(1)

      res = await apiRequest(
        request,
        'GET',
        `/v1/tables/${encodeURIComponent(vendorTable)}/rows?pageSize=10&expand_column_ids=${encodeURIComponent(relCol.column.id)}`,
      )
      expect(res.status()).toBe(200)
      const expanded = await jsonBody<{ rows?: { cells?: Record<string, unknown> }[] }>(res)
      expect(expanded.rows?.[0]?.cells?.[relCol.column.id]).toBeTruthy()

      res = await apiRequest(request, 'GET', `/v1/tables/${encodeURIComponent(vendorTable)}/schema`)
      expect(res.status()).toBe(200)
      const schema = await jsonBody<{ columns?: unknown[] }>(res)
      expect((schema.columns || []).length).toBeGreaterThanOrEqual(5)

      res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(vendorTable)}/rows:import`, {
        format: 1,
        rows: [{ name: 'Imported' }],
      })
      expect(res.status()).toBe(200)

      const hookName = uniqueName('e2e_hook')
      res = await apiRequest(request, 'POST', '/v1/webhooks', {
        name: hookName,
        targetUrl: 'http://127.0.0.1:9/hook',
        tableFilter: vendorTable,
        events: ['records.after.insert'],
        enabled: true,
      })
      expect(res.status()).toBe(200)
      const hook = await jsonBody<{ webhook: { id: string } }>(res)

      res = await apiRequest(request, 'PATCH', `/v1/webhooks/${encodeURIComponent(hook.webhook.id)}`, {
        enabled: false,
      })
      expect(res.status()).toBe(200)

      res = await apiRequest(request, 'DELETE', `/v1/webhooks/${encodeURIComponent(hook.webhook.id)}`)
      expect(res.status()).toBe(200)

      const renamed = vendorTable + '_renamed'
      res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(vendorTable)}:rename`, {
        newName: renamed,
      })
      expect(res.status()).toBe(200)

      await deleteTable(request, renamed)
    } finally {
      await deleteTable(request, orderTable)
    }
  })

  test('row CRUD and bulk operations', async ({ request }) => {
    const table = uniqueName('e2e_crud')
    await createTable(request, table)
    const title = await addColumn(request, table, 'title', 'text', 1)

    try {
      const created = await createRow(request, table, {
        [title.column.id]: { stringValue: 'first' },
      })
      const rowId = created.row.id

      let res = await apiRequest(request, 'PATCH', `/v1/tables/${encodeURIComponent(table)}/rows/${encodeURIComponent(rowId)}`, {
        cells: { [title.column.id]: { stringValue: 'updated' } },
      })
      expect(res.status()).toBe(200)

      res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(table)}/rows:bulkUpsert`, {
        items: [
          { rowId, cells: { [title.column.id]: { stringValue: 'bulk' } } },
          { cells: { [title.column.id]: { stringValue: 'new' } } },
        ],
      })
      expect(res.status()).toBe(200)

      res = await apiRequest(request, 'GET', `/v1/tables/${encodeURIComponent(table)}/rows?pageSize=50`)
      expect(res.status()).toBe(200)
      const listed = await jsonBody<{ rows?: { id: string }[] }>(res)
      expect((listed.rows || []).length).toBeGreaterThanOrEqual(2)

      res = await apiRequest(request, 'POST', `/v1/tables/${encodeURIComponent(table)}/rows:bulkDelete`, {
        rowIds: [rowId],
      })
      expect(res.status()).toBe(200)

      res = await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}/rows/${encodeURIComponent(rowId)}`)
      expect([200, 404]).toContain(res.status())
    } finally {
      await deleteTable(request, table)
    }
  })
})
