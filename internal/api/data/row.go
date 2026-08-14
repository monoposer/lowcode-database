package data

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	svcdata "github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/platform"
)

type Rows struct {
	*httputil.Base
}

func (h *Rows) Query(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	var body svcdata.QueryRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	resp, err := h.Svc.QueryRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) List(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	resp, err := h.Svc.ListRows(r.Context(), listRowsFromQuery(r, tableName))
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Create(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	var body svcdata.CreateRowRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	resp, err := h.Svc.CreateRow(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Update(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	rowID := r.PathValue("rowId")
	var body svcdata.UpdateRowRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	body.RowId = rowID
	resp, err := h.Svc.UpdateRow(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Delete(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	rowID := r.PathValue("rowId")
	resp, err := h.Svc.DeleteRow(r.Context(), &svcdata.DeleteRowRequest{TableName: tableName, RowId: rowID})
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Get(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	rowID := r.PathValue("rowId")
	resp, err := h.Svc.GetRow(r.Context(), &svcdata.GetRowRequest{TableName: tableName, RowId: rowID})
	h.WriteJSON(w, resp, err)
}

func (h *Rows) BulkUpsert(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	var body svcdata.BulkUpsertRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	resp, err := h.Svc.BulkUpsertRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) BulkDelete(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	var body svcdata.BulkDeleteRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	resp, err := h.Svc.BulkDeleteRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Rows) Export(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	var body svcdata.ExportRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
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
	tableName := r.PathValue("tableName")
	var body svcdata.SearchRowsRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.TableName = tableName
	resp, err := h.Svc.SearchRows(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func listRowsFromQuery(r *http.Request, tableName string) *svcdata.ListRowsRequest {
	q := r.URL.Query()
	req := &svcdata.ListRowsRequest{TableName: tableName}
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
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	var body platform.ExecuteQueryRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableName == "" {
		body.TableName = tableName
	}
	body.QueryId = name
	resp, err := h.Svc.ExecuteQuery(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}
