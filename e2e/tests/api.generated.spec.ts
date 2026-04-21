/** AUTO-GENERATED — edit routes.manifest.json and run npm run generate */
import { test, expect } from '@playwright/test'
import { apiRequest, uniqueName, tenantHeaders } from '../lib/api'

test.describe('API smoke (generated GET routes)', () => {
  test('GET /v1/types → types', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/types')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('types')
  })

  test('GET /v1/tables → tables', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/tables')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('tables')
  })

  test('GET /v1/choices → choices', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/choices')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('choices')
  })

  test('GET /v1/relations → relations', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/relations')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('relations')
  })

  test('GET /v1/data-sources → dataSources', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/data-sources')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('dataSources')
  })

  test('GET /v1/webhooks → webhooks', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/webhooks')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('webhooks')
  })

  test('GET /v1/schema/er → diagram', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/schema/er')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('diagram')
  })

  test('GET /v1/database/connection → host', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/database/connection')
    expect(res.status(), await res.text()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('host')
  })

  test('GET /v1/tables without X-Tenant-Id → 400', async ({ request }) => {
    const res = await request.get(`${process.env.API_BASE_URL || 'http://localhost:8080'}/v1/tables`)
    expect(res.status()).toBe(400)
  })
})

test.describe('Built-in types (generated)', () => {
  test('list types includes required builtins', async ({ request }) => {
    const res = await apiRequest(request, 'GET', '/v1/types')
    const body = (await res.json()) as { types?: { id?: string; name?: string }[] }
    const names = new Set((body.types || []).map((t) => t.name || t.id))
    expect(names.has('text')).toBeTruthy()
    expect(names.has('int8')).toBeTruthy()
    expect(names.has('formula')).toBeTruthy()
    expect(names.has('rollup')).toBeTruthy()
    expect(names.has('relation_fk')).toBeTruthy()
  })
})

test.describe('Scalar column types (generated)', () => {
  test('add each scalar type to a table', async ({ request }) => {
    const table = uniqueName('e2e_types')
    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })
    expect(res.status()).toBe(200)
    try {
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_text', typeId: 'text', position: 1,
      })
      expect(res.status(), `type text: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_int8', typeId: 'int8', position: 2,
      })
      expect(res.status(), `type int8: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_double', typeId: 'double', position: 3,
      })
      expect(res.status(), `type double: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_precision', typeId: 'precision', position: 4,
      })
      expect(res.status(), `type precision: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_bool', typeId: 'bool', position: 5,
      })
      expect(res.status(), `type bool: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_timestamptz', typeId: 'timestamptz', position: 6,
      })
      expect(res.status(), `type timestamptz: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_jsonb', typeId: 'jsonb', position: 7,
      })
      expect(res.status(), `type jsonb: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_uuid', typeId: 'uuid', position: 8,
      })
      expect(res.status(), `type uuid: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_int8_array', typeId: 'int8_array', position: 9,
      })
      expect(res.status(), `type int8_array: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'c_text_array', typeId: 'text_array', position: 10,
      })
      expect(res.status(), `type text_array: ${await res.text()}`).toBe(200)
      res = await apiRequest(request, 'GET', `/v1/columns?table_id=${encodeURIComponent(table)}`)
      const cols = (await res.json()) as { columns?: unknown[] }
      expect((cols.columns || []).length).toBeGreaterThanOrEqual(10)
    } finally {
      await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}`)
    }
  })
})

test.describe('Virtual column kinds (generated smoke)', () => {
  test('create table accepts virtual type metadata: formula', async ({ request }) => {
    const table = uniqueName('e2e_formula')
    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })
    expect(res.status()).toBe(200)
    try {
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'score', typeId: 'double', position: 1,
      })
      const scoreCol = (await res.json()) as { column: { id: string } }
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'fx', typeId: 'formula', position: 2,
        config: { expression: '{{score}} * 2' },
      })
      expect(res.status()).toBe(200)
      void scoreCol
    } finally {
      await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}`)
    }
  })

  test('create table accepts virtual type metadata: relationship', async ({ request }) => {
    const table = uniqueName('e2e_relationship')
    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })
    expect(res.status()).toBe(200)
    try {
      // relationship needs target table + link column — covered in workflow spec
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'rel', typeId: 'relationship', position: 1,
        config: { target_table_id: 'missing', link_column_id: '00000000-0000-0000-0000-000000000001', cardinality: 'many' },
      })
      expect(res.status()).toBeGreaterThanOrEqual(400)
    } finally {
      await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}`)
    }
  })

  test('create table accepts virtual type metadata: lookup', async ({ request }) => {
    const table = uniqueName('e2e_lookup')
    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })
    expect(res.status()).toBe(200)
    try {
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'lookup_col', typeId: 'lookup', position: 1,
        config: {},
      })
      expect(res.status()).toBeGreaterThanOrEqual(400)
    } finally {
      await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}`)
    }
  })

  test('create table accepts virtual type metadata: rollup', async ({ request }) => {
    const table = uniqueName('e2e_rollup')
    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })
    expect(res.status()).toBe(200)
    try {
      res = await apiRequest(request, 'POST', '/v1/columns', {
        tableId: table, name: 'rollup_col', typeId: 'rollup', position: 1,
        config: {},
      })
      expect(res.status()).toBeGreaterThanOrEqual(400)
    } finally {
      await apiRequest(request, 'DELETE', `/v1/tables/${encodeURIComponent(table)}`)
    }
  })

})
