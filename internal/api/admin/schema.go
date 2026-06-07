package admin

import (
	"net/http"
	"strings"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/apiv1/query"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
)

type Tables struct {
	*httputil.Base
}

func (h *Tables) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListTables(r.Context(), &apiv1schema.ListTablesRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Tables) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.CreateTableRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateTable(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Tables) Delete(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	if !h.RequireDangerousConfirm(w, r, tableID) {
		return
	}
	resp, err := h.Svc.DeleteTable(r.Context(), &apiv1schema.DeleteTableRequest{Id: tableID})
	h.WriteJSON(w, resp, err)
}

func (h *Tables) Rename(w http.ResponseWriter, r *http.Request) {
	tableID := strings.TrimSuffix(r.PathValue("tableId"), ":rename")
	if tableID == "" || strings.Contains(tableID, "/") {
		http.NotFound(w, r)
		return
	}
	var body apiv1schema.RenameTableRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = tableID
	resp, err := h.Svc.RenameTable(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Tables) GetSchema(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("tableId")
	resp, err := h.Svc.GetTableSchema(r.Context(), &apiv1schema.GetTableSchemaRequest{TableId: tableID})
	h.WriteJSON(w, resp, err)
}

type Columns struct {
	*httputil.Base
}

func (h *Columns) List(w http.ResponseWriter, r *http.Request) {
	req := &apiv1schema.ListColumnsRequest{TableId: httputil.QueryFirst(r, "table_id", "tableId")}
	if req.TableId == "" {
		http.Error(w, "table_id query parameter is required", http.StatusBadRequest)
		return
	}
	resp, err := h.Svc.ListColumns(r.Context(), req)
	h.WriteJSON(w, resp, err)
}

func (h *Columns) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.AddColumnRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	if req.TableId == "" {
		http.Error(w, "tableId is required", http.StatusBadRequest)
		return
	}
	resp, err := h.Svc.AddColumn(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Columns) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body apiv1schema.UpdateColumnRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = id
	if body.TableId == "" {
		body.TableId = httputil.QueryFirst(r, "table_id", "tableId")
	}
	resp, err := h.Svc.UpdateColumn(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Columns) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.RequireDangerousConfirm(w, r, id) {
		return
	}
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.DeleteColumn(r.Context(), &apiv1schema.DeleteColumnRequest{Id: id, TableId: tableID})
	h.WriteJSON(w, resp, err)
}

type Indexes struct {
	*httputil.Base
}

func (h *Indexes) List(w http.ResponseWriter, r *http.Request) {
	req := &apiv1schema.ListIndexesRequest{TableId: httputil.QueryFirst(r, "table_id", "tableId")}
	if req.TableId == "" {
		http.Error(w, "table_id query parameter is required", http.StatusBadRequest)
		return
	}
	resp, err := h.Svc.ListIndexes(r.Context(), req)
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.CreateIndexRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateIndex(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.GetIndex(r.Context(), &apiv1schema.GetIndexRequest{Id: id, TableId: tableID})
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.DeleteIndex(r.Context(), &apiv1schema.DeleteIndexRequest{Id: id, TableId: tableID})
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) Backfill(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.BackfillIndexesFromPG(r.Context(), &apiv1schema.BackfillIndexesRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) BackfillStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.BackfillIndexStatus(r.Context())
	h.WriteJSON(w, resp, err)
}

type ColumnTypes struct {
	*httputil.Base
}

func (h *ColumnTypes) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListColumnTypes(r.Context(), &apiv1schema.ListColumnTypesRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *ColumnTypes) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.CreateColumnTypeRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateColumnType(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *ColumnTypes) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.Svc.GetColumnType(r.Context(), &apiv1schema.GetColumnTypeRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *ColumnTypes) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body apiv1schema.UpdateColumnTypeRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = id
	resp, err := h.Svc.UpdateColumnType(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *ColumnTypes) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.Svc.DeleteColumnType(r.Context(), &apiv1schema.DeleteColumnTypeRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *ColumnTypes) Import(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.ImportTypeCatalogRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.ImportTypeCatalog(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

type Relations struct {
	*httputil.Base
}

func (h *Relations) List(w http.ResponseWriter, r *http.Request) {
	req := &apiv1schema.ListRelationsRequest{TableId: httputil.QueryFirst(r, "table_id", "tableId")}
	resp, err := h.Svc.ListRelations(r.Context(), req)
	h.WriteJSON(w, resp, err)
}

func (h *Relations) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.CreateRelationRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateRelation(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Relations) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sourceTableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.DeleteRelation(r.Context(), &apiv1schema.DeleteRelationRequest{
		SourceTableId: sourceTableID,
		Name:          name,
	})
	h.WriteJSON(w, resp, err)
}

type Queries struct {
	*httputil.Base
}

func (h *Queries) List(w http.ResponseWriter, r *http.Request) {
	req := &query.ListQueriesRequest{TableId: httputil.QueryFirst(r, "table_id", "tableId")}
	resp, err := h.Svc.ListQueries(r.Context(), req)
	h.WriteJSON(w, resp, err)
}

func (h *Queries) Create(w http.ResponseWriter, r *http.Request) {
	var req query.CreateQueryRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateQuery(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Queries) Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.GetQuery(r.Context(), &query.GetQueryRequest{
		TableId: tableID,
		Name:    name,
	})
	h.WriteJSON(w, resp, err)
}

func (h *Queries) Update(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	var body query.UpdateQueryRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableId == "" {
		body.TableId = tableID
	}
	body.Name = name
	resp, err := h.Svc.UpdateQuery(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Queries) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.DeleteQuery(r.Context(), &query.DeleteQueryRequest{
		TableId: tableID,
		Name:    name,
	})
	h.WriteJSON(w, resp, err)
}
