package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/monoposer/lowcode-database/internal/apiv1/platform"
	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/internal/tenant"
)

// Base holds shared HTTP helpers for JSON API handlers.
type Base struct {
	Svc *service.LowcodeService
}

func (b *Base) WithTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenantID(r.Context(), r.Header.Get("X-Tenant-Id"))
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

func QueryFirst(r *http.Request, keys ...string) string {
	q := r.URL.Query()
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func ReadListQuery(r *http.Request, dst any) {
	q := r.URL.Query()
	switch d := dst.(type) {
	case *platform.ListEventDeliveryLogRequest:
		if v := q.Get("limit"); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				d.Limit = n
			}
		}
		d.Status = strings.TrimSpace(q.Get("status"))
	case *platform.ListSchemaAuditRequest:
		if v := q.Get("limit"); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				d.Limit = n
			}
		}
		d.TableId = strings.TrimSpace(q.Get("table_id"))
		if d.TableId == "" {
			d.TableId = strings.TrimSpace(q.Get("tableId"))
		}
	}
}
