package api

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListDataSources() {
	req := &apiv1.ListDataSourcesRequest{}
	if v := h.r.URL.Query().Get("table_id"); v != "" {
		req.TableId = v
	}
	resp, err := h.svc.ListDataSources(h.r.Context(), req)
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateDataSource() {
	var req apiv1.CreateDataSourceRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateDataSource(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleDataSourcesSubtree() {
	rest := strings.TrimPrefix(h.r.URL.Path, "/v1/data-sources/")
	if rest == "" {
		http.NotFound(h.w, h.r)
		return
	}

	// POST /v1/data-sources/{id}:query
	if h.r.Method == http.MethodPost && strings.HasSuffix(rest, ":query") {
		id := strings.TrimSuffix(rest, ":query")
		var body apiv1.QueryDataSourceRequest
		if !h.readJSON(&body) {
			return
		}
		body.DataSourceId = id
		resp, err := h.svc.QueryDataSource(h.r.Context(), &body)
		h.writeJSON(resp, err)
		return
	}

	if strings.Contains(rest, "/") {
		http.NotFound(h.w, h.r)
		return
	}

	switch h.r.Method {
	case http.MethodGet:
		resp, err := h.svc.GetDataSource(h.r.Context(), &apiv1.GetDataSourceRequest{Id: rest})
		h.writeJSON(resp, err)
	case http.MethodPatch:
		var body apiv1.UpdateDataSourceRequest
		if !h.readJSON(&body) {
			return
		}
		body.Id = rest
		resp, err := h.svc.UpdateDataSource(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case http.MethodDelete:
		resp, err := h.svc.DeleteDataSource(h.r.Context(), &apiv1.DeleteDataSourceRequest{Id: rest})
		h.writeJSON(resp, err)
	default:
		http.NotFound(h.w, h.r)
	}
}
