// Package catalog implements choice (PG ENUM) and index catalog operations
// against tenant data databases.
//
//   - service.go — Catalog constructor
//   - loaders.go — LoadColumns and related metadata loaders
//   - choice.go — choice CRUD, resolve, column reference helpers
//   - pg_enum.go — PG ENUM catalog, DDL, migrate, resolve, read
//   - pg_ident.go — PG identifier sanitization helpers
//   - pg_index.go — index introspection and SQL naming
//   - index.go — index CRUD
package catalog
