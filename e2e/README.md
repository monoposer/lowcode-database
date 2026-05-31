# E2E (Playwright)

API end-to-end tests against **Postgres** and the **Go HTTP server** (`:8080`).

Playground UI tests live in the separate repo [lowcode-database-playground](https://github.com/solat/lowcode-database-playground).

## Prerequisites

```bash
make docker-up    # postgres + redis
make migrate      # or rely on docker init
```

## Run

```bash
make e2e
```

Or manually:

```bash
cd e2e
npm install
npx playwright install chromium
npm test              # runs generate + API project
npm run test:api      # API only
```

Environment (optional):

| Variable | Default |
|----------|---------|
| `META_DATABASE_URL` | `postgresql://postgres:postgres@localhost:5432/lowcode_meta` |
| `DEFAULT_TENANT_DATA_DSN` | `postgresql://postgres:postgres@localhost:5432/lowcode_data` |
| `E2E_TENANT_ID` | `default` |
| `API_BASE_URL` | `http://localhost:8080` |
| `E2E_SKIP_MIGRATE` | unset (set `1` to skip migrate in global setup) |

## Auto-generated tests

Edit [`routes.manifest.json`](routes.manifest.json), then:

```bash
npm run generate
```

This writes [`tests/api.generated.spec.ts`](tests/api.generated.spec.ts):

- Smoke **GET** for each platform route in the manifest
- **Scalar column type** matrix (one table, all types)
- **Virtual column** smoke cases
- Required **builtin types** presence check

Hand-written specs:

- [`tests/api-workflow.spec.ts`](tests/api-workflow.spec.ts) — full workflow (tables, relations, formula, data source, webhooks, CRUD)
- [`tests/api-datasource.spec.ts`](tests/api-datasource.spec.ts) — DataSource CRUD + query

Report: `npm run report` after a failed run.
