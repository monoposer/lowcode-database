# lowcode-database Playground

React + Vite UI for manual API testing against the local HTTP server.

## Setup

```bash
cd playground
cp .env.example .env
npm install
```

Start the API server (from repo root), then:

```bash
npm run dev
```

Open the URL Vite prints (default `http://localhost:5173`).

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE` | `http://localhost:8080` | lowcode-database HTTP base URL |

Set **X-Tenant-Id** in the sidebar (default tenant is usually `default`).

## What you can test

| Tab / area | API |
|------------|-----|
| Create / delete / rename table | `POST/DELETE /v1/tables`, `POST ...:rename` |
| Schema → Columns | `GET/POST /v1/columns?table_id=`, `PATCH/DELETE /v1/columns/{id}` |
| Schema → Formula columns | Excel / [pg-formula](https://github.com/SolaTyolo/pg-formula) editor: `config.expression`, `{{column_name}}` refs, formulajs-style functions |
| Schema → Indexes | `GET/POST /v1/indexes?table_id=`, `DELETE /v1/indexes/{pgIndexName}` |
| Rows grid | `GET/POST /v1/tables/{id}/rows`, bulk delete, import |
| DB connection | `GET /v1/database/connection` |

Physical table names match logical names (no `lc_t_` prefix). Indexes are read from PostgreSQL catalog, not a meta mirror table.
