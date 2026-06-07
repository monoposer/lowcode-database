package tenant

import (
	"context"
	"strings"
)

type ctxKey struct{}
type baseKey struct{}
type strongReadKey struct{}

var key ctxKey
var baseIDKey baseKey
var strongReadIDKey strongReadKey

// WithTenantID stores tenant id in context (X-Tenant-Id).
func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

// FromContext extracts tenant id from context if present.
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}

// WithBaseID stores base id in context.
func WithBaseID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, baseIDKey, id)
}

// BaseFromContext extracts base id from context if present.
func BaseFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(baseIDKey).(string); ok {
		return v
	}
	return ""
}

// ResolveTenantID returns X-Tenant-Id from context.
func ResolveTenantID(ctx context.Context) string {
	return strings.TrimSpace(FromContext(ctx))
}

// WithStrongRead forces subsequent DataReadPool calls to use the write DSN
// (replication lag avoidance).
func WithStrongRead(ctx context.Context, strong bool) context.Context {
	return context.WithValue(ctx, strongReadIDKey, strong)
}

// StrongRead reports whether the request asked for primary-consistent reads.
func StrongRead(ctx context.Context) bool {
	v, _ := ctx.Value(strongReadIDKey).(bool)
	return v
}
