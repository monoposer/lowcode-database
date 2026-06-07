// Package apiv1 defines shared REST API primitives (cell Value, SortOrder).
//
// Domain types live in subpackages:
//
//   - schema/     — Table, Column, Index, ColumnType, Relation, ER diagram
//   - row/        — Row entity, CRUD/import/export requests, row JSON helpers
//   - query/      — saved Query admin and execute types (HTTP /queries)
//   - platform/   — Tenant, API keys, observability
//
// Docs: docs/modules/api.md
package apiv1
