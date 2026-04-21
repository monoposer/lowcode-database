package api

import "github.com/solat/lowcode-database/internal/apiv1"

func (h *handler) handleListTypes() {
	resp, err := h.svc.ListTypes(h.r.Context(), &apiv1.ListTypesRequest{})
	h.writeJSON(resp, err)
}
