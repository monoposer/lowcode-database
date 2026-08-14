package api_test

import (
	"bytes"
	"encoding/json"
	"github.com/monoposer/lowcode-database/internal/api"

	svcschema "github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/catalog"

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
	var resp struct {
		Types []*catalog.Type `json:"types"`
	}
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
		"tableName": tableName, "name": "title", "typeId": "text", "position": 1,
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/columns", bytes.NewReader(colBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add column status %d: %s", w.Code, w.Body.String())
	}
	var colResp struct {
		Column *svcschema.Column `json:"column"`
	}
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

	_ = svc.DeleteTable(testutil.Ctx(), tableName)
}

func TestHandlerExecuteQueryNotCreate(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	body := []byte(`{"pageSize":10}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/data/queries/nonexistent_ds?table_name=nonexistent_tbl",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "name is required") {
		t.Fatalf("query route hit CreateQuery: %s", w.Body.String())
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

func TestHandlerUnversionedTablesPathNotFound(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()

	h := api.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/tables", nil)
	req.Header.Set("X-Tenant-Id", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /v1/tables (admin lives under /v1/admin), got %d", w.Code)
	}
}
