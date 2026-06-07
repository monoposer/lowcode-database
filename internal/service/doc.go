// Package service implements business logic split into domain modules.
//
// LowcodeService embeds:
//
//   - schema   — tables/columns DDL, virtual columns, Link relations (docs/modules/schema.md)
//   - catalog  — types, columnTypes, indexes (docs/modules/catalog.md)
//   - data     — row CRUD, query, bulk (docs/modules/data.md)
//   - platform — tenants, keys (docs/modules/platform.md)
//
// Cross-domain reads: meta.Read — docs/modules/meta-shared.md
// Shared kernel: service/shared
//
// Module index: docs/modules/README.md
package service
