package api

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListColumns() {
	req := &apiv1.ListColumnsRequest{}
	if v := h.r.URL.Query().Get("table_id"); v != "" {
		req.TableId = v
	}
	if req.TableId == "" {
		http.Error(h.w, "table_id query parameter is required", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.ListColumns(h.r.Context(), req)
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateColumn() {
	var req apiv1.AddColumnRequest
	if !h.readJSON(&req) {
		return
	}
	if req.TableId == "" {
		http.Error(h.w, "tableId is required", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.AddColumn(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleColumnsSubtree() {
	rest := strings.TrimPrefix(h.r.URL.Path, "/v1/columns/")
	if rest == "" || strings.Contains(rest, "/") {
		http.NotFound(h.w, h.r)
		return
	}
	switch h.r.Method {
	case http.MethodPatch:
		var body apiv1.UpdateColumnRequest
		if !h.readJSON(&body) {
			return
		}
		body.Id = rest
		resp, err := h.svc.UpdateColumn(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case http.MethodDelete:
		resp, err := h.svc.DeleteColumn(h.r.Context(), &apiv1.DeleteColumnRequest{Id: rest})
		h.writeJSON(resp, err)
	default:
		http.NotFound(h.w, h.r)
	}
}
