// Package meta provides cross-domain metadata reads (columns, relationships, indexes).
//
// Domain write logic stays in schema/catalog/graph/platform; callers use meta.New(base).
//
//   - read.go — Read facade constructor
//   - load.go — LoadColumns, relationships, indexes, query ref resolution
package meta
