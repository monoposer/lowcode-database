// Package service implements business logic split into domain packages.
//
// Domains (embedded in LowcodeService):
//
//   - schema — table/column DDL, virtual columns, result types, ER
//   - catalog — PG ENUM choices, index introspection
//   - data — row CRUD, query, saveGraph, bulk, import/export
//   - graph — lc_relations
//   - platform — tenants, API keys, data sources, event sinks, observability
//
// Cross-domain metadata reads: meta.Read (use meta.New(base), not catalog.New/schema.New).
//
// Shared: service/shared (Base, events, cells, result types).
package service
