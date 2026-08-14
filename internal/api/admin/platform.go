package admin

import (
	"fmt"
	"net/http"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	svcplatform "github.com/monoposer/lowcode-database/internal/service/platform"
)

type Platform struct {
	*httputil.Base
}

func (h *Platform) GetDatabaseConnection(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.GetDatabaseConnection(r.Context(), &svcplatform.GetDatabaseConnectionRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req svcplatform.CreateTenantRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateTenant(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req svcplatform.UpdateTenantRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.UpdateTenant(r.Context(), id, &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListWebhooks(r.Context())
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req svcplatform.CreateWebhookRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateWebhook(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.WriteJSON(w, map[string]bool{"ok": true}, h.Svc.DeleteWebhook(r.Context(), id))
}

func (h *Platform) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListAPIKeys(r.Context(), &svcplatform.ListAPIKeysRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req svcplatform.CreateAPIKeyRequest
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
	resp, err := h.Svc.DeleteAPIKey(r.Context(), &svcplatform.DeleteAPIKeyRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.Svc.ListTypes(r.Context())
	h.WriteJSON(w, map[string]any{"types": types}, err)
}

func (h *Platform) ListTenants(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListTenants(r.Context())
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListBases(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListBases(r.Context())
	h.WriteJSON(w, resp, err)
}

func (h *Platform) CreateBase(w http.ResponseWriter, r *http.Request) {
	var req svcplatform.CreateBaseRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateBase(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) ListPGStatStatements(w http.ResponseWriter, r *http.Request) {
	var req svcplatform.ListPGStatStatementsRequest
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			req.Limit = n
		}
	}
	resp, err := h.Svc.ListPGStatStatements(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Platform) Runtime(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.Runtime(r.Context())
	h.WriteJSON(w, resp, err)
}

func (h *Platform) InspectCache(w http.ResponseWriter, r *http.Request) {
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	resp, err := h.Svc.InspectCache(r.Context(), tableName)
	h.WriteJSON(w, resp, err)
}

type ER struct {
	*httputil.Base
}

func (h *ER) GetDiagram(w http.ResponseWriter, r *http.Request) {
	d, err := h.Svc.GetERDiagram(r.Context())
	h.WriteJSON(w, map[string]any{"diagram": d}, err)
}
