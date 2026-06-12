// Package platform implements tenant provisioning, API keys, data sources,
// event sink admin, and observability list APIs (delivery log, schema audit).
//
//   - service.go — Platform constructor
//   - tenant.go, connection.go, apikey.go, type.go
//   - datasource.go — data source CRUD, scan, ResolveDataSourceRef
//   - event_sink.go — event sink CRUD and scan
//   - admin_observability.go — ListEventSchemas, delivery log, schema audit
package platform
