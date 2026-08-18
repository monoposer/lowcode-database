# query-engine module

Query DSL, SQL generation, formula AST evaluation, built-in type registry.

## internal/dsl

Parse filter DSL → structured filter AST (same shape as saved Query `filter` JSON).

```json
{"type":"EQ","attr":"name","val":"BBB"}
{"type":"AND","val":[{"type":"EQ","attr":"status","val":"active"},{"type":"GT","attr":"score","val":10}]}
{"type":"FTS","val":"hello world"}
{"type":"IN","attr":"id","val":["rec_1","rec_2"]}
{"type":"BETWEEN","attr":"created_at","val":["2026-01-01T00:00","2026-12-31T23:59"]}
{"type":"ARRAY_HAS","attr":"items","val":"98ba2647-ffa2-419f-8c2b-fafac6a449be"}
```

**Path:** `internal/dsl/dsl.go`, `params.go`

## internal/query

| File | Role |
|------|------|
| `builder.go` | SELECT / ORDER BY |
| `where.go` | WHERE (DSL → SQL) |

Used by **data** (ListRows, execute saved Query) and **schema** (virtual columns).

## internal/formula

Formula parse and application-layer eval; `{{column}}` goes through [efp](https://github.com/xuri/efp) to an AST, then evaluated in Go.

Supported: `+ - * / ^ &`, comparisons, `%`, `IF` / `AND` / `OR` / `NOT` / `ISBLANK`, `SUM` / `MIN` / `MAX` / `ABS` / `INT` / `ROUND`, `CONCAT` / `TEXT` / `TRIM` / `UPPER` / `LOWER` / `MID` / `LEN`, `DATE` / `DATEVALUE` / `TODAY` / `NOW` / `YEAR` / `MONTH` / `DAY` / `WEEKDAY` / `DATEDIF`.

**Path:** `internal/formula/`

## internal/columntype

Built-in **pgType** registry and tenant columnType specs (see [catalog.md](catalog.md)).

## Data flow

```
Client filter JSON
  → dsl.ParseCached / merge
  → query.BuildWhereWithTypesCached
  → pgx parameterized SQL
  → DataReadPool (or primary when consistency=strong)
```

## Extending

- New filter operators: extend dsl and query.where together
- New base types: `internal/columntype` + docs
