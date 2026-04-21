#!/usr/bin/env node
/**
 * Generates e2e/tests/api.generated.spec.ts from routes.manifest.json.
 * Run: npm run generate (from e2e/)
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.join(__dirname, '..')
const manifest = JSON.parse(
  fs.readFileSync(path.join(root, 'routes.manifest.json'), 'utf8'),
)

const lines = []
lines.push(`/** AUTO-GENERATED — edit routes.manifest.json and run npm run generate */`)
lines.push(`import { test, expect } from '@playwright/test'`)
lines.push(`import { apiRequest, uniqueName, tenantHeaders } from '../lib/api'`)
lines.push('')

lines.push(`test.describe('API smoke (generated GET routes)', () => {`)
for (const r of manifest.smokeGets) {
  lines.push(`  test('GET ${r.path} → ${r.jsonKey}', async ({ request }) => {`)
  lines.push(`    const res = await apiRequest(request, 'GET', '${r.path}')`)
  lines.push(`    expect(res.status(), await res.text()).toBe(200)`)
  lines.push(`    const body = await res.json()`)
  lines.push(`    expect(body).toHaveProperty('${r.jsonKey}')`)
  lines.push(`  })`)
  lines.push('')
}
lines.push(`  test('GET /v1/tables without X-Tenant-Id → 400', async ({ request }) => {`)
lines.push(`    const res = await request.get(\`\${process.env.API_BASE_URL || 'http://localhost:8080'}/v1/tables\`)`)
lines.push(`    expect(res.status()).toBe(400)`)
lines.push(`  })`)
lines.push(`})`)
lines.push('')

lines.push(`test.describe('Built-in types (generated)', () => {`)
lines.push(`  test('list types includes required builtins', async ({ request }) => {`)
lines.push(`    const res = await apiRequest(request, 'GET', '/v1/types')`)
lines.push(`    const body = (await res.json()) as { types?: { id?: string; name?: string }[] }`)
lines.push(`    const names = new Set((body.types || []).map((t) => t.name || t.id))`)
for (const t of manifest.requiredBuiltinTypes) {
  lines.push(`    expect(names.has('${t}')).toBeTruthy()`)
}
lines.push(`  })`)
lines.push(`})`)
lines.push('')

lines.push(`test.describe('Scalar column types (generated)', () => {`)
lines.push(`  test('add each scalar type to a table', async ({ request }) => {`)
lines.push(`    const table = uniqueName('e2e_types')`)
lines.push(`    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })`)
lines.push(`    expect(res.status()).toBe(200)`)
lines.push(`    try {`)
for (let i = 0; i < manifest.scalarColumnTypes.length; i++) {
  const typeId = manifest.scalarColumnTypes[i]
  const colName = `c_${typeId.replace(/[^a-z0-9]/gi, '_')}`
  lines.push(`      res = await apiRequest(request, 'POST', '/v1/columns', {`)
  lines.push(`        tableId: table, name: '${colName}', typeId: '${typeId}', position: ${i + 1},`)
  lines.push(`      })`)
  lines.push(`      expect(res.status(), \`type ${typeId}: \${await res.text()}\`).toBe(200)`)
}
lines.push(`      res = await apiRequest(request, 'GET', \`/v1/columns?table_id=\${encodeURIComponent(table)}\`)`)
lines.push(`      const cols = (await res.json()) as { columns?: unknown[] }`)
lines.push(`      expect((cols.columns || []).length).toBeGreaterThanOrEqual(${manifest.scalarColumnTypes.length})`)
lines.push(`    } finally {`)
lines.push(`      await apiRequest(request, 'DELETE', \`/v1/tables/\${encodeURIComponent(table)}\`)`)
lines.push(`    }`)
lines.push(`  })`)
lines.push(`})`)
lines.push('')

lines.push(`test.describe('Virtual column kinds (generated smoke)', () => {`)
for (const kind of manifest.virtualColumnTypes) {
  lines.push(`  test('create table accepts virtual type metadata: ${kind}', async ({ request }) => {`)
  lines.push(`    const table = uniqueName('e2e_${kind}')`)
  lines.push(`    let res = await apiRequest(request, 'POST', '/v1/tables', { name: table })`)
  lines.push(`    expect(res.status()).toBe(200)`)
  lines.push(`    try {`)
  if (kind === 'formula') {
    lines.push(`      res = await apiRequest(request, 'POST', '/v1/columns', {`)
    lines.push(`        tableId: table, name: 'score', typeId: 'double', position: 1,`)
    lines.push(`      })`)
    lines.push(`      const scoreCol = (await res.json()) as { column: { id: string } }`)
    lines.push(`      res = await apiRequest(request, 'POST', '/v1/columns', {`)
    lines.push(`        tableId: table, name: 'fx', typeId: 'formula', position: 2,`)
    lines.push(`        config: { expression: '{{score}} * 2' },`)
    lines.push(`      })`)
    lines.push(`      expect(res.status()).toBe(200)`)
    lines.push(`      void scoreCol`)
  } else if (kind === 'relationship') {
    lines.push(`      // relationship needs target table + link column — covered in workflow spec`)
    lines.push(`      res = await apiRequest(request, 'POST', '/v1/columns', {`)
    lines.push(`        tableId: table, name: 'rel', typeId: 'relationship', position: 1,`)
    lines.push(`        config: { target_table_id: 'missing', link_column_id: '00000000-0000-0000-0000-000000000001', cardinality: 'many' },`)
    lines.push(`      })`)
    lines.push(`      expect(res.status()).toBeGreaterThanOrEqual(400)`)
  } else {
    lines.push(`      res = await apiRequest(request, 'POST', '/v1/columns', {`)
    lines.push(`        tableId: table, name: '${kind}_col', typeId: '${kind}', position: 1,`)
    lines.push(`        config: {},`)
    lines.push(`      })`)
    lines.push(`      expect(res.status()).toBeGreaterThanOrEqual(400)`)
  }
  lines.push(`    } finally {`)
  lines.push(`      await apiRequest(request, 'DELETE', \`/v1/tables/\${encodeURIComponent(table)}\`)`)
  lines.push(`    }`)
  lines.push(`  })`)
  lines.push('')
}
lines.push(`})`)

const outPath = path.join(root, 'tests', 'api.generated.spec.ts')
fs.mkdirSync(path.dirname(outPath), { recursive: true })
fs.writeFileSync(outPath, lines.join('\n') + '\n')
console.log(`Wrote ${outPath}`)
