package api

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListIndexes() {
	req := &apiv1.ListIndexesRequest{}
	if v := h.r.URL.Query().Get("table_id"); v != "" {
		req.TableId = v
	}
	if req.TableId == "" {
		http.Error(h.w, "table_id query parameter is required", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.ListIndexes(h.r.Context(), req)
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateIndex() {
	var req apiv1.CreateIndexRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateIndex(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleIndexesSubtree() {
	rest := strings.TrimPrefix(h.r.URL.Path, "/v1/indexes/")
	if rest == "" || strings.Contains(rest, "/") {
		http.NotFound(h.w, h.r)
		return
	}
	switch h.r.Method {
	case http.MethodGet:
		resp, err := h.svc.GetIndex(h.r.Context(), &apiv1.GetIndexRequest{Id: rest})
		h.writeJSON(resp, err)
	case http.MethodDelete:
		resp, err := h.svc.DeleteIndex(h.r.Context(), &apiv1.DeleteIndexRequest{Id: rest})
		h.writeJSON(resp, err)
	default:
		http.NotFound(h.w, h.r)
	}
}
