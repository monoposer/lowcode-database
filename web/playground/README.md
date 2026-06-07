# lowcode-database playground

Vite + React + AG Grid UI for this repo’s HTTP JSON API (`/v1/*`).

Migrated from [examples/lowcode-database-playground](https://github.com/monoposer/examples/tree/main/lowcode-database-playground).

## Pages (hash routes)

| Route | Notes |
|-------|-------|
| `#/editor` | Tables, columns, rows (AG Grid) |
| `#/choices` | Choice / ENUM |
| `#/queries` | Saved Query |
| `#/settings` | Tenant / connection |

## Run

```bash
# repo root
make docker-up
make migrate
make run                 # API http://localhost:8080 (default embed calc worker)

make playground-dev      # UI http://localhost:5173
```

Or point another `cmd/server` at this UI via `VITE_API_BASE`.

| Variable | Default |
|----------|---------|
| `VITE_API_BASE` | `http://localhost:8080` |
| `VITE_DEFAULT_TENANT_ID` | `default` |

Headers: **X-Tenant-Id**, optional **X-Base-Id**.
