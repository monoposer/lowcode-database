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

    await expect(page.getByText('hello playground')).toBeVisible({ timeout: 15_000 })

    const cell = page.locator('.ag-row .ag-cell').filter({ hasText: 'hello playground' }).first()
    await cell.click()
    await cell.press('Control+a')
    await cell.pressSequentially('edited row')
    await cell.press('Enter')
    await expect(page.getByText('edited row')).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('reload-btn').click()
    await expect(page.getByText('edited row')).toBeVisible({ timeout: 15_000 })
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

    await page.getByTestId('edit-column-double_score').click()
    await expect(page.getByTestId('column-edit-panel')).toBeVisible()
    const editFormula = page.locator('.column-edit-panel .formula-input')
    await editFormula.fill('={{score}} * 2')
    await page.getByTestId('save-column-btn').click()
    await expect(page.getByText('{{score}} * 2')).toBeVisible({ timeout: 15_000 })
  })

  test('edit column name in schema tab', async ({ page }) => {
    const tableName = uniqueName('ui_edit_col')

    await page.getByTestId('create-table-name').fill(tableName)
    await page.getByTestId('create-table-btn').click()
    await page.getByTestId('table-select').selectOption({ label: tableName })

    await page.getByTestId('tab-schema').click()
    await page.getByTestId('add-column-name').fill('old_name')
    await page.getByTestId('add-column-type').selectOption('text')
    await page.getByTestId('add-column-btn').click()
    await expect(page.getByRole('cell', { name: 'old_name' })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('edit-column-old_name').click()
    await page.getByTestId('edit-column-name').fill('new_name')
    await page.getByTestId('save-column-btn').click()
    await expect(page.getByRole('cell', { name: 'new_name' })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('cell', { name: 'old_name' })).not.toBeVisible()
  })

  test('create index in playground', async ({ page }) => {
    const tableName = uniqueName('ui_idx')

    await page.getByTestId('create-table-name').fill(tableName)
    await page.getByTestId('create-table-btn').click()
    await page.getByTestId('table-select').selectOption({ label: tableName })

    await page.getByTestId('tab-schema').click()
    await page.getByTestId('add-column-name').fill('code')
    await page.getByTestId('add-column-type').selectOption('text')
    await page.getByTestId('add-column-btn').click()
    await expect(page.getByRole('cell', { name: 'code' })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('create-index-name').fill('idx_code')
    await page.locator('.idx-cols input').first().check()
    await page.getByTestId('create-index-btn').click()
    await expect(page.getByRole('cell', { name: 'idx_code' })).toBeVisible({ timeout: 15_000 })
  })

  test('create saved query and run in playground', async ({ page }) => {
    const tableName = uniqueName('ui_ds')
    const dsName = uniqueName('ui_view')

    await page.getByTestId('create-table-name').fill(tableName)
    await page.getByTestId('create-table-btn').click()
    await page.getByTestId('table-select').selectOption({ label: tableName })

    await page.getByTestId('tab-schema').click()
    await page.getByTestId('add-column-name').fill('title')
    await page.getByTestId('add-column-type').selectOption('text')
    await page.getByTestId('add-column-btn').click()
    await expect(page.getByRole('cell', { name: 'title' })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('add-column-name').fill('active')
    await page.getByTestId('add-column-type').selectOption('bool')
    await page.getByTestId('add-column-btn').click()
    await expect(page.getByRole('cell', { name: 'active' })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('tab-rows').click()
    await page.getByLabel('title').fill('Visible row')
    await page.locator('.new-row-panel label').filter({ hasText: 'active' }).locator('input').fill('true')
    await page.getByTestId('insert-row-btn').click()
    await expect(page.getByText('Visible row')).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('tab-queries').click()
    await page.getByTestId('create-query-name').fill(dsName)
    await page.getByTestId('create-query-filter-val').fill('true')
    await page.getByTestId('create-query-btn').click()
    await expect(page.getByRole('cell', { name: dsName })).toBeVisible({ timeout: 15_000 })

    await page.getByTestId(`run-query-${dsName}`).click()
    await expect(page.getByTestId('query-result')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('Visible row')).toBeVisible()
  })
})
