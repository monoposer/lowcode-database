# E2E (Playwright)

End-to-end tests against a running **Postgres**, the **Go HTTP server** (`:8080`), and the **Playground** preview (`:5173`).

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
npm test              # runs generate + all projects
npm run test:api      # API only
npm run test:ui       # Playground UI only
```

Environment (optional):

| Variable | Default |
|----------|---------|
| `META_DATABASE_URL` | `postgresql://postgres:postgres@localhost:5432/lowcode_meta` |
| `DEFAULT_TENANT_DATA_DSN` | `postgresql://postgres:postgres@localhost:5432/lowcode_data` |
| `E2E_TENANT_ID` | `default` |
| `API_BASE_URL` | `http://localhost:8080` |
| `PLAYGROUND_URL` | `http://localhost:5173` |
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
- [`tests/playground.spec.ts`](tests/playground.spec.ts) — Playground UI (table/column/row + formula editor)

Report: `npm run report` after a failed run.
