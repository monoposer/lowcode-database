package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/solat/lowcode-database/internal/api/admin"
	"github.com/solat/lowcode-database/internal/api/data"
	"github.com/solat/lowcode-database/internal/api/httputil"
	"github.com/solat/lowcode-database/internal/service"
)

const (
	V1Prefix    = "/v1"
	AdminPrefix = V1Prefix + "/admin"
	DataPrefix  = V1Prefix + "/data"
)

// NewHandler returns an http.Handler for /v1/admin/* and /v1/data/* JSON APIs.
func NewHandler(svc *service.LowcodeService) http.Handler {
	base := &httputil.Base{Svc: svc}

	platform := &admin.Platform{Base: base}
	event := &admin.Event{Base: base}
	er := &admin.ER{Base: base}
	tables := &admin.Tables{Base: base}
	columns := &admin.Columns{Base: base}
	indexes := &admin.Indexes{Base: base}
	choices := &admin.Choices{Base: base}
	relations := &admin.Relations{Base: base}
	adminDS := &admin.DataSources{Base: base}

	rows := &data.Rows{Base: base}
	dataDS := &data.DataSources{Base: base}

	r := chi.NewRouter()
	r.Use(base.WithTenant)
	r.Use(base.EnsureWritable)

	r.Route(AdminPrefix, func(r chi.Router) {
		r.Get("/database/connection", platform.GetDatabaseConnection)
		r.Post("/tenants", platform.CreateTenant)

		r.Get("/api-keys", platform.ListAPIKeys)
		r.Post("/api-keys", platform.CreateAPIKey)
		r.Delete("/api-keys/{id}", platform.DeleteAPIKey)

		r.Get("/types", platform.ListTypes)

		r.Get("/events/schemas", event.ListSchemas)
		r.Get("/events/envelope-schema", event.ListSchemas)
		r.Get("/events/delivery-log", event.ListDeliveryLog)
		r.Get("/schema-audit", event.ListSchemaAudit)

		r.Get("/event-sinks", event.ListSinks)
		r.Post("/event-sinks", event.CreateSink)
		r.Delete("/event-sinks/{id}", event.DeleteSink)
		r.Patch("/event-sinks/{id}", event.UpdateSink)

		r.Get("/tables", tables.List)
		r.Post("/tables", tables.Create)
		r.Delete("/tables/{tableId}", tables.Delete)
		r.Post("/tables/{tableId}:rename", tables.Rename)
		r.Get("/tables/{tableId}/schema", tables.GetSchema)

		r.Get("/columns", columns.List)
		r.Post("/columns", columns.Create)
		r.Patch("/columns/{id}", columns.Update)
		r.Delete("/columns/{id}", columns.Delete)

		r.Get("/indexes", indexes.List)
		r.Post("/indexes", indexes.Create)
		r.Get("/indexes/{id}", indexes.Get)
		r.Delete("/indexes/{id}", indexes.Delete)

		r.Get("/schema/er", er.GetDiagram)

		r.Get("/choices", choices.List)
		r.Post("/choices", choices.Create)
		r.Get("/choices/{id}", choices.Get)
		r.Patch("/choices/{id}", choices.Update)
		r.Delete("/choices/{id}", choices.Delete)

		r.Get("/relations", relations.List)
		r.Post("/relations", relations.Create)
		r.Delete("/relations/{name}", relations.Delete)

		r.Get("/data-sources", adminDS.List)
		r.Post("/data-sources", adminDS.Create)
		r.Get("/data-sources/{name}", adminDS.Get)
		r.Patch("/data-sources/{name}", adminDS.Update)
		r.Delete("/data-sources/{name}", adminDS.Delete)
	})

	r.Route(DataPrefix, func(r chi.Router) {
		r.Get("/tables/{tableId}/rows", rows.List)
		r.Post("/tables/{tableId}/rows", rows.Create)
		r.Post("/tables/{tableId}/rows:query", rows.Query)
		r.Post("/tables/{tableId}/rows:bulkUpsert", rows.BulkUpsert)
		r.Post("/tables/{tableId}/rows:saveGraph", rows.SaveGraph)
		r.Post("/tables/{tableId}/rows:bulkDelete", rows.BulkDelete)
		r.Post("/tables/{tableId}/rows:import", rows.Import)
		r.Post("/tables/{tableId}/rows:export", rows.Export)
		r.Post("/tables/{tableId}/rows:search", rows.Search)
		r.Patch("/tables/{tableId}/rows/{rowId}", rows.Update)
		r.Delete("/tables/{tableId}/rows/{rowId}", rows.Delete)

		r.Post("/data-sources/{name}:query", dataDS.Query)
	})

	return r
}
