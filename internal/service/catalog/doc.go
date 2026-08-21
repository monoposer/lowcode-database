// Package catalog is the type & catalog module: pgTypes, tenant columnTypes,
// index catalog, and column metadata loading.
//
// Files:
//
//   - types.go — ListTypes (pgType + tenant columnType); columnTypeToAPIType
//   - column_type.go — lc_column_types CRUD
//   - index.go — index catalog reads
//
// Related: internal/columntype (built-in pgType registry and columnType spec).
package catalog
