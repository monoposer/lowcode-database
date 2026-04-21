package api

import (
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListRelations() {
	req := &apiv1.ListRelationsRequest{}
	if v := h.r.URL.Query().Get("table_id"); v != "" {
		req.TableId = v
	}
	resp, err := h.svc.ListRelations(h.r.Context(), req)
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateRelation() {
	var req apiv1.CreateRelationRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateRelation(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleDeleteRelation() {
	id := strings.TrimPrefix(h.r.URL.Path, "/v1/relations/")
	resp, err := h.svc.DeleteRelation(h.r.Context(), &apiv1.DeleteRelationRequest{Id: id})
	h.writeJSON(resp, err)
}
