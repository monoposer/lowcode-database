import { test, expect } from '@playwright/test'
import { uniqueName } from '../lib/api'

const tenantId = process.env.E2E_TENANT_ID || 'default'

test.describe('Playground UI', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Playground' })).toBeVisible()
    await page.getByLabel('X-Tenant-Id').fill(tenantId)
  })

  test('create table, add column, insert row, schema tab', async ({ page }) => {
    const tableName = uniqueName('ui_tbl')

    await page.getByTestId('create-table-name').fill(tableName)
    await page.getByTestId('create-table-btn').click()

    await page.getByTestId('table-select').selectOption({ label: tableName })

    await page.getByTestId('tab-schema').click()
    await expect(page.getByRole('heading', { name: 'Columns' })).toBeVisible()

    const colName = 'title'
    await page.getByTestId('add-column-name').fill(colName)
    await page.getByTestId('add-column-type').selectOption('text')
    await page.getByTestId('add-column-btn').click()

    await expect(page.getByRole('cell', { name: colName })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('tab-rows').click()
    await page.getByLabel(colName).fill('hello playground')
    await page.getByTestId('insert-row-btn').click()

    await page.getByTestId('reload-btn').click()
    await expect(page.getByText('hello playground')).toBeVisible({ timeout: 15_000 })
  })

  test('formula column editor visible when type is formula', async ({ page }) => {
    const tableName = uniqueName('ui_formula')

    await page.getByTestId('create-table-name').fill(tableName)
    await page.getByTestId('create-table-btn').click()
    await page.getByTestId('table-select').selectOption({ label: tableName })

    await page.getByTestId('tab-schema').click()
    await page.getByTestId('add-column-name').fill('score')
    await page.getByTestId('add-column-type').selectOption('double')
    await page.getByTestId('add-column-btn').click()
    await expect(page.getByRole('cell', { name: 'score' })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('add-column-name').fill('double_score')
    await page.getByTestId('add-column-type').selectOption('formula')
    await expect(page.getByRole('heading', { name: /New formula/i })).toBeVisible()

    const formulaInput = page.locator('.formula-input')
    await formulaInput.fill('=SUM({{score}}, 1)')
    await page.getByRole('button', { name: 'score' }).click()
    await page.getByTestId('add-column-btn').click()

    await expect(page.getByRole('cell', { name: 'double_score' })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('{{score}}')).toBeVisible()
  })
})
