package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListTables() {
	resp, err := h.svc.ListTables(h.r.Context(), &apiv1.ListTablesRequest{})
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateTable() {
	var req apiv1.CreateTableRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateTable(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleTablesSubtree() {
	rest := strings.TrimPrefix(h.r.URL.Path, "/v1/tables/")
	if rest == "" {
		http.NotFound(h.w, h.r)
		return
	}

	// POST /v1/tables/{id}:rename
	if h.r.Method == http.MethodPost && strings.HasSuffix(rest, ":rename") {
		h.handleRenameTable(strings.TrimSuffix(rest, ":rename"))
		return
	}
	// DELETE /v1/tables/{id}
	if h.r.Method == http.MethodDelete && !strings.Contains(rest, "/") && !strings.Contains(rest, ":") {
		resp, err := h.svc.DeleteTable(h.r.Context(), &apiv1.DeleteTableRequest{Id: rest})
		h.writeJSON(resp, err)
		return
	}

	parts := strings.SplitN(rest, "/", 2)
	tableID := parts[0]
	if tableID == "" || len(parts) < 2 {
		http.NotFound(h.w, h.r)
		return
	}
	h.handleTableResource(tableID, parts[1])
}

func (h *handler) handleRenameTable(tableID string) {
	if tableID == "" || strings.Contains(tableID, "/") {
		http.NotFound(h.w, h.r)
		return
	}
	var body apiv1.RenameTableRequest
	if !h.readJSON(&body) {
		return
	}
	body.Id = tableID
	resp, err := h.svc.RenameTable(h.r.Context(), &body)
	h.writeJSON(resp, err)
}

func (h *handler) handleTableResource(tableID, tail string) {
	switch {
	case tail == "schema" && h.r.Method == http.MethodGet:
		resp, err := h.svc.GetTableSchema(h.r.Context(), &apiv1.GetTableSchemaRequest{TableId: tableID})
		h.writeJSON(resp, err)

	case tail == "rows:query" && h.r.Method == http.MethodPost:
		var body apiv1.QueryRowsRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		resp, err := h.svc.QueryRows(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case tail == "rows" && h.r.Method == http.MethodGet:
		resp, err := h.svc.ListRows(h.r.Context(), h.listRowsFromQuery(tableID))
		h.writeJSON(resp, err)
	case tail == "rows" && h.r.Method == http.MethodPost:
		var body apiv1.CreateRowRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		resp, err := h.svc.CreateRow(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case strings.HasPrefix(tail, "rows/"):
		h.handleTableRowByID(tableID, strings.TrimPrefix(tail, "rows/"))
	case tail == "rows:bulkUpsert" && h.r.Method == http.MethodPost:
		var body apiv1.BulkUpsertRowsRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		resp, err := h.svc.BulkUpsertRows(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case tail == "rows:bulkDelete" && h.r.Method == http.MethodPost:
		var body apiv1.BulkDeleteRowsRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		resp, err := h.svc.BulkDeleteRows(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case tail == "rows:import" && h.r.Method == http.MethodPost:
		var body apiv1.ImportRowsRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		resp, err := h.svc.ImportRows(h.r.Context(), &body)
		h.writeJSON(resp, err)

	default:
		http.NotFound(h.w, h.r)
	}
}

func (h *handler) handleTableRowByID(tableID, rowID string) {
	if strings.Contains(rowID, "/") {
		http.NotFound(h.w, h.r)
		return
	}
	switch h.r.Method {
	case http.MethodPatch:
		var body apiv1.UpdateRowRequest
		if !h.readJSON(&body) {
			return
		}
		body.TableId = tableID
		body.RowId = rowID
		resp, err := h.svc.UpdateRow(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case http.MethodDelete:
		resp, err := h.svc.DeleteRow(h.r.Context(), &apiv1.DeleteRowRequest{TableId: tableID, RowId: rowID})
		h.writeJSON(resp, err)
	default:
		http.NotFound(h.w, h.r)
	}
}

func (h *handler) listRowsFromQuery(tableID string) *apiv1.ListRowsRequest {
	q := h.r.URL.Query()
	req := &apiv1.ListRowsRequest{TableId: tableID}
	if v := firstQueryVal(q, "pageSize", "page_size"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			req.PageSize = int32(n)
		}
	}
	if v := firstQueryVal(q, "pageToken", "page_token"); v != "" {
		req.PageToken = v
	}
	req.ExpandColumnIds = append(append([]string{}, q["expand_column_ids"]...), q["expandColumnIds"]...)
	if v := firstQueryVal(q, "expand_paths", "expandPaths"); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				req.ExpandPaths = append(req.ExpandPaths, p)
			}
		}
	}
	return req
}

func firstQueryVal(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}
