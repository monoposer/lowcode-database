# Record storage and async calc (link_ref + calc_queue)

> **Status: implemented.** Rows live in the shard `record` table (`data jsonb`). Link instances live in `link_ref`. formula / lookup / rollup caches are refreshed by `calc_queue` (DB queue + worker).

Tenant shards and LIST partitions: [virtual-records.md](virtual-records.md) (`record` evolved from `virtual_records`: `tenant_id` = tenant, `vt_id` = logical-table partition key, `record_id` = row id).

## Storage model

| Object | DB | Notes |
|--------|----|-------|
| **field** | Meta `lc_columns` | `type_id`: scalar / `link` / `formula` / `lookup` / `rollup` |
| **record** | Data shard | `data jsonb` holds user input + calc cache; **does not** store link id arrays |
| **link_ref** | Data shard | Link instance edges |
| **calc_queue** | Data shard | Authoritative task queue (optional MQ wake-up, no payload) |

`record.data` calc field shape:

```json
{
  "title": "manual input",
  "fld_formula": { "value": 120, "_cache_status": "valid" },
  "fld_lookup": { "value": "Alice", "_cache_status": "valid" }
}
```

`_cache_status`: `valid` | `error`. Enqueue / pending / retry **do not** write the record; `version + 1` only on user edit or successful worker cache write.

## Consistency

- **List**: read `record.data` cache; batch-check `calc_queue status=0`; expose row-level `pending` as “updating”.
- **Detail** `GET /v1/data/tables/{id}/rows/{rowId}`: compute DAG live; if it differs from cache and there is no pending, enqueue only — do not mutate the record.
- Eventually consistent; no cross-row strong consistency.

## Read / write

1. User changes a scalar field → update `record.data` + `version` → enqueue DAG downstream into `calc_queue`.
2. User changes a Link → update `link_ref` only (two rows if bidirectional) → enqueue this record.
3. Remote record change → reverse-lookup `from_record_id` on `link_ref` and batch-enqueue (lookup fan-out; rollup usually fans out 1).
4. Worker: `SELECT … FOR UPDATE SKIP LOCKED` → compute formula/lookup/rollup → **partial** write of data keys + optimistic lock.

## API planes

| Prefix | Role |
|--------|------|
| `/v1/admin/*` | **Meta API**: tables/fields (including link/formula/lookup/rollup definitions) |
| `/v1/data/*` | Row CRUD / list cache / live detail |

`internal/service/calc.Worker` polls every active data store from **inside `cmd/server`**: one shared `calc_queue` per DSN, plus each dedicated tenant's `{tenant_id}_calc_queue`. Multi-tenant: `SKIP LOCKED`. There is no public calc HTTP.

## Field options (`lc_columns.config`)

- **link**: `to_table_name` (or `target_table_name`), `bidirectional`, `inverse_field_id`, `cardinality`
- **formula**: `expression`, `deps` (may be inferred from `{{col}}`)
- **lookup**: `link_field_id` / `relation_column_id`, `target_field_id` / `target_column_id`
- **rollup**: same + `aggregation` / `aggregate` (`sum`|`count`|`max`|`min`|`avg`)

## Constraints

- Do not replace the whole `data` object to write calc fields; do not bump version on enqueue.
- Many child lookups of the same parent produce queue fan-out; avoid at the product layer.
- List filter/sort depends on cache; if dirty cache is unacceptable, use live detail only.
