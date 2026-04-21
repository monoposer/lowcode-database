package api

import "github.com/solat/lowcode-database/internal/apiv1"

func (h *handler) handleGetDatabaseConnection() {
	resp, err := h.svc.GetDatabaseConnection(h.r.Context(), &apiv1.GetDatabaseConnectionRequest{})
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateTenant() {
	var req apiv1.CreateTenantRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateTenant(h.r.Context(), &req)
	h.writeJSON(resp, err)
}
