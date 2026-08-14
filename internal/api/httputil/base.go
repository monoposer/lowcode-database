package httputil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/pkg/tenant"
)

// Base holds shared HTTP helpers for JSON API handlers.
type Base struct {
	Svc *service.LowcodeService
}

func (b *Base) WithTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get("X-Tenant-Id")
		if tid == "" {
			tid = r.Header.Get("X-Tenant-ID")
		}
		ctx := tenant.WithTenantID(r.Context(), tid)
		baseID := r.Header.Get("X-Base-Id")
		if baseID == "" {
			baseID = r.Header.Get("X-Base-ID")
		}
		ctx = tenant.WithBaseID(ctx, baseID)
		if isStrongConsistency(r.Header.Get("X-Read-Consistency"), r.URL.Query().Get("consistency")) {
			ctx = tenant.WithStrongRead(ctx, true)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (b *Base) EnsureWritable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.Svc != nil && b.Svc.Schema != nil && b.Svc.Schema.B.Tenants != nil {
			if err := b.Svc.Schema.B.Tenants.EnsureWritable(r.Context(), r.Method, r.URL.Path); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Base) ReadJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		return true
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return true
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func (b *Base) WriteJSON(w http.ResponseWriter, v any, err error) {
	if err != nil {
		b.WriteErr(w, err)
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (b *Base) WriteErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	code := http.StatusBadRequest
	if strings.Contains(strings.ToLower(msg), "not found") {
		code = http.StatusNotFound
	}
	http.Error(w, msg, code)
}

func isStrongConsistency(vals ...string) bool {
	for _, v := range vals {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "strong", "primary", "write":
			return true
		}
	}
	return false
}

// WithStrongReadFromValue marks the request for primary reads (replication-lag avoidance).
func WithStrongReadFromValue(ctx context.Context, consistency string) context.Context {
	if isStrongConsistency(consistency) {
		return tenant.WithStrongRead(ctx, true)
	}
	return ctx
}

func QueryFirst(r *http.Request, keys ...string) string {
	q := r.URL.Query()
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}
