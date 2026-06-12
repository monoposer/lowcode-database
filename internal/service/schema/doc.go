// Package schema implements table/column metadata CRUD, relationships, lookups,
// formulas, and result-type resolution.
//
//   - service.go — Schema service constructor
//   - table.go, table_id.go — table CRUD and ID column DDL
//   - column_mutate.go — AddColumn, UpdateColumn
//   - column_crud.go, column_public.go — list/delete/alter + index read
//   - column_virtual.go — config, ref, relationship column kinds
//   - virtual.go — lookup write specs + formula validation
//   - fk.go — FK column config
//   - result_type.go — result type resolution
//   - loaders.go — metadata loaders and ER diagram
package schema
