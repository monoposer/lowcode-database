import { defineConfig, devices } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080'
const PLAYGROUND_BASE = process.env.PLAYGROUND_URL || 'http://localhost:5173'
const repoRoot = process.env.E2E_REPO_ROOT || '..'

const dbEnv = {
  META_DATABASE_URL:
    process.env.META_DATABASE_URL ||
    'postgresql://postgres:postgres@localhost:5432/lowcode_meta',
  DEFAULT_TENANT_DATA_DSN:
    process.env.DEFAULT_TENANT_DATA_DSN ||
    'postgresql://postgres:postgres@localhost:5432/lowcode_data',
  DEFAULT_TENANT_ID: process.env.E2E_TENANT_ID || 'default',
  HTTP_ADDR: process.env.HTTP_ADDR || ':8080',
}

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  timeout: 120_000,
  globalSetup: './global-setup.ts',
  use: {
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'api',
      testMatch: /api.*\.spec\.ts/,
      use: {
        baseURL: API_BASE,
        extraHTTPHeaders: {
          'X-Tenant-Id': process.env.E2E_TENANT_ID || 'default',
        },
      },
    },
    {
      name: 'playground',
      testMatch: /playground\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        baseURL: PLAYGROUND_BASE,
      },
    },
  ],
  webServer: [
    {
      command: 'go run ./cmd/server',
      cwd: repoRoot,
      env: dbEnv,
      url: `${API_BASE}/v1/types`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: 'npm run build && npx vite preview --port 5173 --host 127.0.0.1',
      cwd: `${repoRoot}/playground`,
      url: PLAYGROUND_BASE,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
    },
  ],
})
