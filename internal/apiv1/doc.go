// Package apiv1 defines shared REST API primitives (cell Value, SortOrder).
//
// Domain types live in subpackages:
//
//   - schema/     — Table, Column, Index, Choice, Relation, ER diagram
//   - row/        — Row entity, CRUD/import/export requests, row JSON helpers
//   - datasource/ — DataSource admin and query types
//   - graph/      — SaveGraph nested relationship save
//   - platform/   — Tenant, API keys, built-in types, event sinks, observability
package apiv1
