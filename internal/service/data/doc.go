// Package data implements row CRUD, DSL query execution, relationship expand,
// saveGraph, bulk operations, and import/export.
//
//   - service.go — Data service constructor
//   - row.go — row CRUD + relationship expand
//   - row_lookup.go — lookup column read path
//   - query.go — QueryRows, QueryDataSource, SearchRows
//   - query_exec.go — query run, plan, virtual columns
//   - lookup.go — lookup write + linked filter
//   - columns_select.go — column selection for queries
//   - savegraph.go, bulk.go, import.go
package data
