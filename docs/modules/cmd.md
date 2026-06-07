# cmd module

## Paths

| Entry | Role |
|-------|------|
| `cmd/server` | Only runtime: `/v1/admin/*`, `/v1/data/*`, `/v1/worker/*` + in-process calc worker |
| `cmd/migrate` | Apply embed `migrations/` to meta + every unique data write DSN |

## server startup

```
config.Load()
  → postgres.NewTenantManager
  → EventBus + LowcodeService
  → authn (optional API Key)
  → api.NewHandler → HTTP_ADDR (:8080)
  → calc.Worker.Run + IndexMigrate
```

## migrate

```bash
make migrate                    # meta + data
make migrate-meta               # meta only
make docker-migrate             # compose run --rm migrate
```

The server does **not** auto-migrate at runtime.

## Dependency direction

`cmd/*` depends only on `internal/*`, `pkg/*`, and `migrations/`. Other packages must not import `cmd`.
