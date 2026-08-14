// Package shared holds cross-domain helpers: Base dependencies, cells, config,
// result-type constants, cache invalidation, and EventBus publish on EmitEvent.
//
//   - base.go — Base struct and NewBase
//   - events.go — EmitEvent: EventBus publish (webhooks consume the bus)
//   - cache.go — metadata cache invalidation
//   - helpers.go — tenant, value utilities, cell helpers
//   - pg.go — PG type helpers
//   - config.go — column config parsing + identifier validation
//   - result_type.go — result type id classify/infer
//   - virtual.go — lookup + formula virtual-column helpers
//   - column_type_migrate.go — column type migration helpers
//   - types.go — ColumnMeta, RelationshipColumn, LookupWriteSpec
package shared
