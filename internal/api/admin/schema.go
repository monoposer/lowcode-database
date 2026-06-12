package admin

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/api/httputil"
	"github.com/solat/lowcode-database/internal/apiv1/datasource"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
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
	resp, err := h.Svc.GetIndex(r.Context(), &apiv1schema.GetIndexRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *Indexes) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.Svc.DeleteIndex(r.Context(), &apiv1schema.DeleteIndexRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

type Choices struct {
	*httputil.Base
}

func (h *Choices) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.Svc.ListChoices(r.Context(), &apiv1schema.ListChoicesRequest{})
	h.WriteJSON(w, resp, err)
}

func (h *Choices) Create(w http.ResponseWriter, r *http.Request) {
	var req apiv1schema.CreateChoiceRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateChoice(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *Choices) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.Svc.GetChoice(r.Context(), &apiv1schema.GetChoiceRequest{Id: id})
	h.WriteJSON(w, resp, err)
}

func (h *Choices) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body apiv1schema.UpdateChoiceRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	body.Id = id
	resp, err := h.Svc.UpdateChoice(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *Choices) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.Svc.DeleteChoice(r.Context(), &apiv1schema.DeleteChoiceRequest{Id: id})
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

type DataSources struct {
	*httputil.Base
}

func (h *DataSources) List(w http.ResponseWriter, r *http.Request) {
	req := &datasource.ListDataSourcesRequest{TableId: httputil.QueryFirst(r, "table_id", "tableId")}
	resp, err := h.Svc.ListDataSources(r.Context(), req)
	h.WriteJSON(w, resp, err)
}

func (h *DataSources) Create(w http.ResponseWriter, r *http.Request) {
	var req datasource.CreateDataSourceRequest
	if !h.ReadJSON(w, r, &req) {
		return
	}
	resp, err := h.Svc.CreateDataSource(r.Context(), &req)
	h.WriteJSON(w, resp, err)
}

func (h *DataSources) Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.GetDataSource(r.Context(), &datasource.GetDataSourceRequest{
		TableId: tableID,
		Name:    name,
	})
	h.WriteJSON(w, resp, err)
}

func (h *DataSources) Update(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	var body datasource.UpdateDataSourceRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	if body.TableId == "" {
		body.TableId = tableID
	}
	body.Name = name
	resp, err := h.Svc.UpdateDataSource(r.Context(), &body)
	h.WriteJSON(w, resp, err)
}

func (h *DataSources) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tableID := httputil.QueryFirst(r, "table_id", "tableId")
	resp, err := h.Svc.DeleteDataSource(r.Context(), &datasource.DeleteDataSourceRequest{
		TableId: tableID,
		Name:    name,
	})
	h.WriteJSON(w, resp, err)
}
