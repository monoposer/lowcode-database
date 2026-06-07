# Roadmap

Items **not in this version**. Do not add product APIs, meta tables, or docs for them until they ship. Current behavior stays in [architecture/](architecture/system.md) and [modules/](modules/README.md).

Schema and types today: Admin CRUD (`/v1/admin/tables`, `/columns`, `/indexes`, `/column-types`) and `pkg/typespec` for pgType / DOMAIN. There is **no** portable schema bundle import.

---

## Schema bundle import

**Not in this version.** Removed: `POST /v1/admin/schema:import`, `pkg/schemaspec`, `docs/spec/`.

A later version may add a declarative pack (tables + columns + indexes + relations + queries) that validates then calls the existing Admin APIs in order. Until then, create objects one request at a time (or script the Admin API).

`POST /v1/admin/column-types:import` (TypeCatalog) remains; it only upserts tenant DOMAINs.

---

## Plugins

**Not in this version.** No WASM host, plugin Admin CRUD, `/v1/data/rpc`, write-path triggers, or `plugin.{name}.{type}` columns.

Custom validation today: `POST /v1/admin/column-types` (`pkg/typespec` / PG DOMAIN). React to writes in the application after Admin/Data APIs succeed (or use webhooks).

A later version may add a four-tier model: subscriptions, before/after row triggers, RPC, custom plugin types.

---

## Graph query

**Not in this version.** No `expandColumnIds` / `expandPaths`, no `POST .../rows:saveGraph`, no multi-hop traversal.

Still available: Link columns (`link_ref`), `GET/POST/DELETE /v1/admin/relations`, `GET /v1/admin/schema/er`, flat row CRUD with link IDs.

A later version may add expand-on-list, nested graph writes, and path queries.

---

## Choice / ENUM

**Not in this version.** No `lc_choices`, no `CREATE TYPE … AS ENUM` as a product feature, no `/v1/admin/choices`.

Callers may store bounded sets as `text` / `jsonb` plus app validation, or create ENUM/DOMAIN themselves on the data DB. Built-in **pgType** and tenant **columnType** still apply.

---

## Authorization (RBAC)

**Not in this version.** No route permission middleware, `AUTHZ_*`, or grant tables. Isolation is `X-Tenant-Id`. Optional **authentication**: `X-Api-Key`.

Fine-grained roles belong in the BFF / control plane. A later version may add route → resource checks (file or HTTP authorizer). Do not set `AUTHZ_*`.

---

## Process split / calc ownership

One `cmd/server` polls every shard `calc_queue` (`SKIP LOCKED`). A later version may elect a calc owner (Redis) or add `-role=api|calc` on the **same** binary.

---

## Events: at-least-once

`EmitEvent` is after commit; publish failures are dropped. Memory bus is single-instance; multi-replica needs `EVENT_BUS=redis`. A later version may add an outbox in the data transaction if at-least-once after commit is required.
