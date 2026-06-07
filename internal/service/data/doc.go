// Package data implements row CRUD, DSL query execution, bulk operations, and import/export.
//
//   - service.go — Data service constructor
//   - row.go — ListRows + insert helper
//   - row_lookup.go — lookup column read path
//   - query.go — QueryRows, ExecuteQuery, SearchRows
//   - query_exec.go — query run, plan, virtual columns
//   - lookup.go — lookup joins + linked filter
//   - columns_select.go — column selection for queries
//   - bulk.go, import.go
package data
