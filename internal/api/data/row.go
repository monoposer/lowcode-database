package data

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/apiv1/datasource"
	"github.com/monoposer/lowcode-database/internal/apiv1/graph"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
)

type Rows struct {
	*httputil.Base
}

func (h *Rows) Query(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.QueryRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.QueryRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) List(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	resp, err := h.Svc.ListRows(r.Context(), listRowsFromQuery(r, tableID))
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Create(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.CreateRowRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.CreateRow(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Update(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	rowID := r.PathValue("rowId")
	var body row.UpdateRowRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	body.RowId = rowID
	resp, err := h.Svc.UpdateRow(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Delete(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	rowID := r.PathValue("rowId")
	resp, err := h.Svc.DeleteRow(r.Context(), &row.DeleteRowRequest{TableId: tableID, RowId: rowID})
	h.WriteJSON(w, resp, err)
}

func (h *Rows) BulkUpsert(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.BulkUpsertRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.BulkUpsertRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) SaveGraph(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body graph.SaveGraphRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.SaveGraph(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) BulkDelete(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.BulkDeleteRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.BulkDeleteRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Import(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.ImportRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.ImportRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Export(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.ExportRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.ExportRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Search(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.SearchRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	resp, err := h.Svc.SearchRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func listRowsFromQuery(r *http.Request, tableID string) *row.ListRowsRequest {
	q := r.URL.Query()
	req := &row.ListRowsRequest{TableId: tableID}
	if v := queryFirst(q, "pageSize", "page_size"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			req.PageSize = int32(n)
		}
	}
	if v := queryFirst(q, "pageToken", "page_token"); v != "" {
		req.PageToken = v
	}
	req.ExpandColumnIds = append(append([]string{}, q["expand_column_ids"]...), q["expandColumnIds"]...)
	if v := queryFirst(q, "expand_paths", "expandPaths"); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				req.ExpandPaths = append(req.ExpandPaths, p)
			}
		}
	}
	return req
}

func queryFirst(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

type DataSources struct {
	*httputil.Base
}

func (h *DataSources) Query(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(r.PathValue("name"), ":query")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	var body datasource.QueryDataSourceRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableId == "" {
		body.TableId = tableID
	}
	body.DataSourceId = name
	resp, err := h.Svc.QueryDataSource(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}
