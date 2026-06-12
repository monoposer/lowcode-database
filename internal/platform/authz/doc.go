// Package authz maps HTTP requests to RBAC checks (file or HTTP driver).
//
//   - authz.go — Authorizer interface, NewFromConfig, HTTP driver
//   - headers.go — configurable subject HTTP headers (sub + roles)
//   - routes.go — RequestFromHTTP, admin/data path → Action mapping
//   - file.go — FileAuthorizer (JSON role → permission grants)
//   - middleware.go — HTTP middleware wrapper
package authz
