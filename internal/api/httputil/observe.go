package httputil

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/monoposer/lowcode-database/internal/telemetry"
	"github.com/monoposer/lowcode-database/internal/tenant"
)

func (b *Base) Observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if b == nil || b.Svc == nil || b.Svc.Schema == nil || b.Svc.Schema.B == nil {
			return
		}
		tel := b.Svc.Schema.B.Telemetry
		if tel == nil {
			return
		}
		op := r.Method + " " + r.URL.Path
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			op = r.Method + " " + rc.RoutePattern()
		}
		tableID := r.PathValue("tableId")
		kind := "http.request.duration_ms"
		switch {
		case strings.Contains(op, "/admin/"):
			kind = "meta.request.duration_ms"
		case strings.Contains(op, "/data/"):
			kind = "data.request.duration_ms"
		case strings.Contains(op, "/worker/calc"):
			kind = "calc.task.duration_ms"
		}
		labels := telemetry.Labels(tenant.ResolveTenantID(r.Context()), tenant.BaseFromContext(r.Context()), tableID, op)
		tel.RecordHistogram(kind, float64(time.Since(start).Milliseconds()), labels)
		tel.IncCounter("http.requests", labels)
	})
}
