package admin

import (
	"net/http"
	"strings"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/platform"
	"github.com/monoposer/lowcode-database/internal/service/schema"
)

type Tables struct {
	*httputil.Base
}

func (h *Tables) List(w http.ResponseWriter, r *http.Request) {
	tables, err := h.Svc.ListTables(r.Context())
	h.WriteJSON(w, map[string]any{"tables": tables}, err)
}

func (h *Tables) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.Table
	if !h.ReadJSON(w, r, &req) {
		return
	}
	t, err := h.Svc.CreateTable(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"table": t}, err)
}

func (h *Tables) Delete(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	if !h.RequireDangerousConfirm(w, r, tableName) {
		return
	}
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteTable(r.Context(), tableName))
}

func (h *Tables) Rename(w http.ResponseWriter, r *http.Request) {
	tableName := strings.TrimSuffix(r.PathValue("tableName"), ":rename")
	if tableName == "" || strings.Contains(tableName, "/") {
		http.NotFound(w, r)
		return
	}
	var body struct {
		NewName string `json:"newName"`
	}
	if !h.ReadJSON(w, r, &body) {
		return
	}
	t, err := h.Svc.RenameTable(r.Context(), tableName, body.NewName)
	h.WriteJSON(w, map[string]any{"table": t}, err)
}

func (h *Tables) GetSchema(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("tableName")
	tbl, cols, idxs, err := h.Svc.GetTableSchema(r.Context(), tableName)
	h.WriteJSON(w, map[string]any{"table": tbl, "columns": cols, "indexes": idxs}, err)
}

type Columns struct {
	*httputil.Base
}

func (h *Columns) List(w http.ResponseWriter, r *http.Request) {
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	if tableName == "" {
		http.Error(w, "table_name query parameter is required", http.StatusBadRequest)
		return
	}
	cols, err := h.Svc.ListColumns(r.Context(), tableName)
	h.WriteJSON(w, map[string]any{"columns": cols}, err)
}

func (h *Columns) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.Column
	if !h.ReadJSON(w, r, &req) {
		return
	}
	if req.TableName == "" {
		http.Error(w, "tableName is required", http.StatusBadRequest)
		return
	}
	c, err := h.Svc.AddColumn(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"column": c}, err)
}

func (h *Columns) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		TableName    string         `json:"tableName"`
		Name       string         `json:"name"`
		Label      string         `json:"label"`
		TypeId     string         `json:"typeId"`
		IsNullable *bool          `json:"isNullable"`
		Position   int32          `json:"position"`
		Config     map[string]any `json:"config"`
	}
	if !h.ReadJSON(w, r, &body) {
		return
	}
	tableName := body.TableName
	if tableName == "" {
		tableName = httputil.QueryFirst(r, "table_name", "tableName")
	}
	c, err := h.Svc.UpdateColumn(r.Context(), &schema.Column{
		Id: id, TableName: tableName, Name: body.Name, Label: body.Label,
		TypeId: body.TypeId, Position: body.Position, Config: body.Config,
	}, body.IsNullable)
	h.WriteJSON(w, map[string]any{"column": c}, err)
}

func (h *Columns) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.RequireDangerousConfirm(w, r, id) {
		return
	}
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteColumn(r.Context(), tableName, id))
}

type Indexes struct {
	*httputil.Base
}

func (h *Indexes) List(w http.ResponseWriter, r *http.Request) {
	req := httputil.QueryFirst(r, "table_name", "tableName")
	if req == "" {
		http.Error(w, "table_name query parameter is required", http.StatusBadRequest)
		return
	}
	indexes, err := h.Svc.ListIndexes(r.Context(), req)
	h.WriteJSON(w, map[string]any{"indexes": indexes}, err)
}

func (h *Indexes) Create(w http.ResponseWriter, r *http.Request) {
	var req catalog.Index
	if !h.ReadJSON(w, r, &req) {
		return
	}
	idx, err := h.Svc.CreateIndex(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"index": idx}, err)
}

func (h *Indexes) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	idx, err := h.Svc.GetIndex(r.Context(), tableName, id)
	h.WriteJSON(w, map[string]any{"index": idx}, err)
}

func (h *Indexes) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteIndex(r.Context(), tableName, id))
}

func (h *Indexes) Backfill(w http.ResponseWriter, r *http.Request) {
	n, err := h.Svc.BackfillIndexesFromPG(r.Context())
	h.WriteJSON(w, map[string]any{"indexesCreated": n}, err)
}

func (h *Indexes) BackfillStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.BackfillIndexStatus(r.Context())
	h.WriteJSON(w, resp, err)
}

type ColumnTypes struct {
	*httputil.Base
}

func (h *ColumnTypes) List(w http.ResponseWriter, r *http.Request) {
	cts, err := h.Svc.ListColumnTypes(r.Context())
	h.WriteJSON(w, map[string]any{"columnTypes": cts}, err)
}

func (h *ColumnTypes) Create(w http.ResponseWriter, r *http.Request) {
	var req catalog.ColumnTypeDef
	if !h.ReadJSON(w, r, &req) {
		return
	}
	ct, err := h.Svc.CreateColumnType(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"columnType": ct}, err)
}

func (h *ColumnTypes) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ct, err := h.Svc.GetColumnType(r.Context(), id)
	h.WriteJSON(w, map[string]any{"columnType": ct}, err)
}

func (h *ColumnTypes) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body catalog.ColumnTypeDef
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = id
	ct, err := h.Svc.UpdateColumnType(r.Context(), &body)
	h.WriteJSON(w, map[string]any{"columnType": ct}, err)
}

func (h *ColumnTypes) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteColumnType(r.Context(), id))
}

func (h *ColumnTypes) Import(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Catalog *columntype.TypeCatalog `json:"catalog"`
	}
	if !h.ReadJSON(w, r, &req) {
		return
	}
	n, err := h.Svc.ImportTypeCatalog(r.Context(), req.Catalog)
	h.WriteJSON(w, map[string]any{"columnTypesCreated": n}, err)
}

type Relations struct {
	*httputil.Base
}

func (h *Relations) List(w http.ResponseWriter, r *http.Request) {
	rels, err := h.Svc.ListRelations(r.Context(), httputil.QueryFirst(r, "table_name", "tableName"))
	h.WriteJSON(w, map[string]any{"relations": rels}, err)
}

func (h *Relations) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.Relation
	if !h.ReadJSON(w, r, &req) {
		return
	}
	rel, err := h.Svc.CreateRelation(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"relation": rel}, err)
}

func (h *Relations) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sourceTableName := httputil.QueryFirst(r, "table_name", "tableName")
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteRelation(r.Context(), sourceTableName, name))
}

type Queries struct {
	*httputil.Base
}

func (h *Queries) List(w http.ResponseWriter, r *http.Request) {
	qs, err := h.Svc.ListQueries(r.Context(), httputil.QueryFirst(r, "table_name", "tableName"))
	h.WriteJSON(w, map[string]any{"queries": qs}, err)
}

func (h *Queries) Create(w http.ResponseWriter, r *http.Request) {
	var req platform.Query
	if !h.ReadJSON(w, r, &req) {
		return
	}
	q, err := h.Svc.CreateQuery(r.Context(), &req)
	h.WriteJSON(w, map[string]any{"query": q}, err)
}

func (h *Queries) Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	q, err := h.Svc.GetQuery(r.Context(), tableName, name)
	h.WriteJSON(w, map[string]any{"query": q}, err)
}

func (h *Queries) Update(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	var body platform.Query
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableName == "" {
		body.TableName = tableName
	}
	body.Name = name
	q, err := h.Svc.UpdateQuery(r.Context(), &body)
	h.WriteJSON(w, map[string]any{"query": q}, err)
}

func (h *Queries) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableName := httputil.QueryFirst(r, "table_name", "tableName")
	h.WriteJSON(w, map[string]any{}, h.Svc.DeleteQuery(r.Context(), tableName, name))
}
