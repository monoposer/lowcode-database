import { execSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

export default async function globalSetup() {
  const meta =
    process.env.META_DATABASE_URL ||
    'postgresql://postgres:postgres@localhost:5432/lowcode_meta'
  const data =
    process.env.DEFAULT_TENANT_DATA_DSN ||
    process.env.DATA_DATABASE_URL ||
    'postgresql://postgres:postgres@localhost:5432/lowcode_data'

  process.env.META_DATABASE_URL = meta
  process.env.DEFAULT_TENANT_DATA_DSN = data
  process.env.API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:8080'
  process.env.PLAYGROUND_URL = process.env.PLAYGROUND_URL || 'http://localhost:5173'
  process.env.E2E_REPO_ROOT = repoRoot

  if (process.env.E2E_SKIP_MIGRATE !== '1') {
    try {
      execSync('go run ./cmd/migrate -target meta', {
        cwd: repoRoot,
        env: { ...process.env, META_DATABASE_URL: meta },
        stdio: 'inherit',
      })
      execSync('go run ./cmd/migrate -target data', {
        cwd: repoRoot,
        env: { ...process.env, DATA_DATABASE_URL: data },
        stdio: 'inherit',
      })
    } catch (e) {
      console.warn('[e2e global-setup] migrate failed (is Postgres up?):', e)
      if (process.env.CI) throw e
    }
  }

  execSync('node scripts/generate-api-tests.mjs', {
    cwd: path.join(repoRoot, 'e2e'),
    stdio: 'inherit',
  })
}
