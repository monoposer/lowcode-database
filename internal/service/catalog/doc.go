// Package catalog is the type & catalog module: pgTypes, tenant columnTypes,
// index catalog, and column metadata loading.
//
// Files:
//
//   - types.go — ListTypes (pgType + columnType); columnTypeToAPIType
//   - column_type.go — lc_column_types CRUD, column type DDL (pkg/typespec)
//   - index.go — index catalog reads
//
// Related: internal/columntype (built-in pgType registry), pkg/typespec (portable column type spec).
package catalog
