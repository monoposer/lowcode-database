package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/monoposer/lowcode-database/internal/api/admin"
	"github.com/monoposer/lowcode-database/internal/api/data"
	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/service"
)

const (
	V1Prefix    = "/v1"
	AdminPrefix = V1Prefix + "/admin"
	DataPrefix  = V1Prefix + "/data"
)

// NewHandler serves /v1/admin/* and /v1/data/* (the only production HTTP surface).
func NewHandler(svc *service.LowcodeService) http.Handler {
	base := &httputil.Base{Svc: svc}
	r := chi.NewRouter()
	mountCommon(r, base)
	mountAdmin(r, base)
	mountData(r, base)
	return r
}

func mountCommon(r chi.Router, base *httputil.Base) {
	r.Use(base.WithTenant)
	r.Use(base.EnsureWritable)
	r.Use(base.Observe)
	if base.Svc != nil && base.Svc.Schema != nil && base.Svc.Schema.B != nil && base.Svc.Schema.B.HTTPMiddleware != nil {
		r.Use(base.Svc.Schema.B.HTTPMiddleware)
	}
}

func mountAdmin(r chi.Router, base *httputil.Base) {
	platform := &admin.Platform{Base: base}
	er := &admin.ER{Base: base}
	tables := &admin.Tables{Base: base}
	columns := &admin.Columns{Base: base}
	indexes := &admin.Indexes{Base: base}
	columnTypes := &admin.ColumnTypes{Base: base}
	relations := &admin.Relations{Base: base}
	adminQueries := &admin.Queries{Base: base}

	r.Route(AdminPrefix, func(r chi.Router) {
		r.Get("/database/connection", platform.GetDatabaseConnection)
		r.Get("/tenants", platform.ListTenants)
		r.Post("/tenants", platform.CreateTenant)
		r.Patch("/tenants/{id}", platform.UpdateTenant)

		r.Get("/webhooks", platform.ListWebhooks)
		r.Post("/webhooks", platform.CreateWebhook)
		r.Delete("/webhooks/{id}", platform.DeleteWebhook)

		r.Get("/bases", platform.ListBases)
		r.Post("/bases", platform.CreateBase)

		r.Get("/api-keys", platform.ListAPIKeys)
		r.Post("/api-keys", platform.CreateAPIKey)
		r.Delete("/api-keys/{id}", platform.DeleteAPIKey)

		r.Get("/types", platform.ListTypes)

		r.Get("/pg-stat-statements", platform.ListPGStatStatements)
		r.Get("/runtime", platform.Runtime)
		r.Get("/cache:inspect", platform.InspectCache)

		r.Get("/tables", tables.List)
		r.Post("/tables", tables.Create)
		r.Delete("/tables/{tableName}", tables.Delete)
		r.Post("/tables/{tableName}:rename", tables.Rename)
		r.Get("/tables/{tableName}/schema", tables.GetSchema)

		r.Get("/columns", columns.List)
		r.Post("/columns", columns.Create)
		r.Patch("/columns/{id}", columns.Update)
		r.Delete("/columns/{id}", columns.Delete)

		r.Get("/indexes", indexes.List)
		r.Post("/indexes", indexes.Create)
		r.Get("/indexes/{id}", indexes.Get)
		r.Delete("/indexes/{id}", indexes.Delete)
		r.Post("/indexes:backfill", indexes.Backfill)
		r.Get("/indexes:backfill-status", indexes.BackfillStatus)

		r.Get("/schema/er", er.GetDiagram)

		r.Get("/column-types", columnTypes.List)
		r.Post("/column-types", columnTypes.Create)
		r.Get("/column-types/{id}", columnTypes.Get)
		r.Patch("/column-types/{id}", columnTypes.Update)
		r.Delete("/column-types/{id}", columnTypes.Delete)
		r.Post("/column-types:import", columnTypes.Import)

		r.Get("/relations", relations.List)
		r.Post("/relations", relations.Create)
		r.Delete("/relations/{name}", relations.Delete)

		r.Get("/queries", adminQueries.List)
		r.Post("/queries", adminQueries.Create)
		r.Get("/queries/{name}", adminQueries.Get)
		r.Patch("/queries/{name}", adminQueries.Update)
		r.Delete("/queries/{name}", adminQueries.Delete)
	})
}

func mountData(r chi.Router, base *httputil.Base) {
	rows := &data.Rows{Base: base}
	dataQueries := &data.Queries{Base: base}

	r.Route(DataPrefix, func(r chi.Router) {
		r.Get("/tables/{tableName}/rows", rows.List)
		r.Post("/tables/{tableName}/rows", rows.Create)
		r.Post("/tables/{tableName}/rows:query", rows.Query)
		r.Post("/tables/{tableName}/rows:bulkUpsert", rows.BulkUpsert)
		r.Post("/tables/{tableName}/rows:bulkDelete", rows.BulkDelete)
		r.Post("/tables/{tableName}/rows:export", rows.Export)
		r.Post("/tables/{tableName}/rows:search", rows.Search)
		r.Patch("/tables/{tableName}/rows/{rowId}", rows.Update)
		r.Get("/tables/{tableName}/rows/{rowId}", rows.Get)
		r.Delete("/tables/{tableName}/rows/{rowId}", rows.Delete)

		r.Post("/queries/{name}", dataQueries.Query)
	})
}
