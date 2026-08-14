package httputil

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/monoposer/lowcode-database/pkg/telemetry"
	"github.com/monoposer/lowcode-database/pkg/tenant"
)

func (b *Base) Observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		op := r.Method + " " + r.URL.Path
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			op = r.Method + " " + rc.RoutePattern()
		}
		tableName := r.PathValue("tableName")
		kind := "http.request.duration_ms"
		switch {
		case strings.Contains(op, "/admin/"):
			kind = "meta.request.duration_ms"
		case strings.Contains(op, "/data/"):
			kind = "data.request.duration_ms"
		}
		labels := telemetry.Labels(tenant.ResolveTenantID(r.Context()), tenant.BaseFromContext(r.Context()), tableName, op)
		telemetry.RecordHistogram(kind, float64(time.Since(start).Milliseconds()), labels)
		telemetry.IncCounter("http.requests", labels)
	})
}
