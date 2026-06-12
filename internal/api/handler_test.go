package api_test

import (
	"bytes"
	"encoding/json"
	"github.com/monoposer/lowcode-database/internal/api"
	"github.com/monoposer/lowcode-database/internal/apiv1/platform"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"

	"github.com/monoposer/lowcode-database/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerListTypes(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/types", nil)
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp platform.ListTypesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Types) == 0 {
		t.Fatal("expected types")
	}
}

func TestHandlerCreateTableAndQuery(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	tableName := testutil.UniqueName("api_tbl")

	createBody, _ := json.Marshal(map[string]any{"name": tableName})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/tables", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create table status %d: %s", w.Code, w.Body.String())
	}

	colBody, _ := json.Marshal(map[string]any{
		"tableId": tableName, "name": "title", "typeId": "text", "position": 1,
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/columns", bytes.NewReader(colBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add column status %d: %s", w.Code, w.Body.String())
	}
	var colResp apiv1schema.AddColumnResponse
	_ = json.Unmarshal(w.Body.Bytes(), &colResp)

	rowBody, _ := json.Marshal(map[string]any{
		"cells": map[string]any{
			colResp.Column.Id: map[string]any{"stringValue": "hello"},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/data/tables/"+tableName+"/rows", bytes.NewReader(rowBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create row status %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/data/tables/"+tableName+"/rows?pageSize=10", nil)
	req.Header.Set("X-Tenant-Id", "test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list rows status %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/schema/er", nil)
	req.Header.Set("X-Tenant-Id", "test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("er diagram status %d: %s", w.Code, w.Body.String())
	}

	_, _ = svc.DeleteTable(testutil.Ctx(), &apiv1schema.DeleteTableRequest{Id: tableName})
}

func TestHandlerDataSourceQueryNotCreate(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	body := []byte(`{"pageSize":10}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/data/data-sources/nonexistent_ds:query?table_id=nonexistent_tbl",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "name is required") {
		t.Fatalf("query route hit CreateDataSource: %s", w.Body.String())
	}
}

func TestHandlerMissingTenant(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/tables", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerSaveGraph(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	h := api.NewHandler(svc)
	orderTable := testutil.UniqueName("order")
	itemTable := testutil.UniqueName("order_item")

	createTable := func(name string) {
		body, _ := json.Marshal(map[string]any{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/tables", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-Id", "test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("create table %s: %d %s", name, w.Code, w.Body.String())
		}
	}
	createTable(orderTable)
	createTable(itemTable)
	defer func() {
		_, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: orderTable})
		_, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: itemTable})
	}()

	addCol := func(tableID, name, typeID string, position int, config map[string]any) apiv1schema.Column {
		payload := map[string]any{
			"tableId": tableID, "name": name, "typeId": typeID, "position": position,
		}
		if config != nil {
			payload["config"] = config
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/columns", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-Id", "test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("add column %s.%s: %d %s", tableID, name, w.Code, w.Body.String())
		}
		var resp apiv1schema.AddColumnResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return *resp.Column
	}

	addCol(orderTable, "amount", "precision", 1, nil)
	addCol(itemTable, "qty", "int8", 1, nil)
	addCol(itemTable, "goods_id", "text", 2, nil)
	orderLinkCol := addCol(itemTable, "order_id", "uuid", 3, nil)
	addCol(orderTable, "items", "relationship", 2, map[string]any{
		"target_table_id": itemTable,
		"link_column_id":  orderLinkCol.Id,
		"cardinality":     "many",
	})

	saveBody := []byte(`{
		"amount": 42,
		"items": [
			{ "qty": 1, "goods_id": "g1" }
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/data/tables/"+orderTable+"/rows:saveGraph", bytes.NewReader(saveBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("saveGraph status %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Fatalf("missing root row: %+v", resp)
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item, got %+v", resp["items"])
	}
}

func TestHandlerLegacyPathsNotFound(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/tables", nil)
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for legacy path, got %d", w.Code)
	}
}
