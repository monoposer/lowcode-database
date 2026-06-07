# query-engine module

Query DSL, SQL generation, formula AST evaluation, built-in type registry.

## internal/dsl

Parse filter DSL → structured filter AST (same shape as saved Query `filter` JSON).

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

Built-in **base** type registry (see [catalog.md](catalog.md)).

Vs **pkg/typespec**: typespec defines the canonical list + Domain DDL; columntype is runtime `Resolve`/`List`.

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
- New base types: columntype + typespec/base_types.go + docs
