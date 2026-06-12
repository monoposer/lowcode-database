package admin

import (
	"net/http"

	"github.com/solat/lowcode-database/internal/api/httputil"
	"github.com/solat/lowcode-database/internal/apiv1/platform"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
)

type Platform struct {
	*httputil.Base
}

func (h *Platform) GetDatabaseConnection(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.GetDatabaseConnection(r.Context(), &platform.GetDatabaseConnectionRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req platform.CreateTenantRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateTenant(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListAPIKeys(r.Context(), &platform.ListAPIKeysRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req platform.CreateAPIKeyRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateAPIKey(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	resp, err := h.Svc.DeleteAPIKey(r.Context(), &platform.DeleteAPIKeyRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListTypes(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListTypes(r.Context(), &platform.ListTypesRequest{})
	h.WriteJSON(w, resp, err)
}

type Event struct {
	*httputil.Base
}

func (h *Event) ListSchemas(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListEventSchemas(r.Context(), &platform.ListEventSchemasRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Event) ListSinks(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListEventSinks(r.Context(), &platform.ListEventSinksRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Event) CreateSink(w http.ResponseWriter, r *http.Request) {
	var req platform.CreateEventSinkRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateEventSink(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Event) DeleteSink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	resp, err := h.Svc.DeleteEventSink(r.Context(), &platform.DeleteEventSinkRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *Event) UpdateSink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	var body platform.UpdateEventSinkRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = id
	resp, err := h.Svc.UpdateEventSink(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Event) ListDeliveryLog(w http.ResponseWriter, r *http.Request) {
	var req platform.ListEventDeliveryLogRequest
	httputil.ReadListQuery(r, &req)
	resp, err := h.Svc.ListEventDeliveryLog(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Event) ListSchemaAudit(w http.ResponseWriter, r *http.Request) {
	var req platform.ListSchemaAuditRequest
	httputil.ReadListQuery(r, &req)
	resp, err := h.Svc.ListSchemaAudit(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

type ER struct {
	*httputil.Base
}

func (h *ER) GetDiagram(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.GetERDiagram(r.Context(), &apiv1schema.GetERDiagramRequest{})
	h.WriteJSON(w, resp, err)
}
