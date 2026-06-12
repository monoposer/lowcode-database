// Package api provides HTTP routing and handlers for /v1/admin/* and /v1/data/* JSON APIs.
//
// Layout:
//
//   - routes.go           — chi router registration (single route table)
//   - httputil/base.go    — JSON read/write, tenant + writable middleware
//   - admin/              — admin plane handlers (platform, schema, datasource)
//   - data/               — data plane handlers (rows, datasource query)
//   - openapi.go          — /openapi/* and /swagger UI
package api
