package data

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/apiv1/query"
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

func (h *Rows) Get(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	rowID := r.PathValue("rowId")
	resp, err := h.Svc.GetRow(r.Context(), &row.GetRowRequest{TableId: tableID, RowId: rowID})
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
	resp, err := h.Svc.ImportRowsStream(r.Context(), tableID, nil, r.Body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Export(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	var body row.ExportRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableId = tableID
	format := body.Format
	if format == "" {
		format = "json"
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	if _, err := h.Svc.ExportRowsTo(r.Context(), &body, w); err != nil {
		h.WriteErr(w, err)
	}
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
	if v := queryFirst(q, "consistency"); v != "" {
		req.Consistency = v
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

type Queries struct {
	*httputil.Base
}

func (h *Queries) Query(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	var body query.ExecuteQueryRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableId == "" {
		body.TableId = tableID
	}
	body.QueryId = name
	resp, err := h.Svc.ExecuteQuery(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}
