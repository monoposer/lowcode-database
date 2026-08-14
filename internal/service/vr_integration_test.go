package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

func TestVirtualRecordsCRUD(t *testing.T) {
	svc, cleanup := testutil.SetupIntegrationVR(t)
	defer cleanup()
	ctx := testutil.CtxVR()

	table := testutil.UniqueName("vr_orders")
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: table}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	titleCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: table, Name: "title", TypeId: "text", Position: 2,
		Config: map[string]any{"enable_fulltext": true, "need_index": true},
	})
	if err != nil {
		t.Fatalf("add column: %v", err)
	}
	_ = titleCol

	str := "hello virtual records"
	created, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: table,
		Cells:   map[string]*shared.Value{"title": {StringValue: &str}},
	})
	if err != nil {
		t.Fatalf("create row: %v", err)
	}
	if created.Row == nil || created.Row.Id == "" {
		t.Fatalf("missing row id")
	}

	q, err := svc.QueryRows(ctx, &data.QueryRowsRequest{
		TableName: table,
		Filter:  map[string]any{"field": "title", "op": "eq", "value": str},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(q.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(q.Rows))
	}

	search, err := svc.SearchRows(ctx, &data.SearchRowsRequest{TableName: table, Query: "virtual"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(search.Rows) < 1 {
		t.Fatalf("fts search expected hits, got %d", len(search.Rows))
	}

	idx, err := svc.CreateIndex(ctx, &catalog.Index{
		TableName: table, Name: "by_title", ColumnIds: []string{titleCol.Id},
	})
	if err != nil {
		t.Fatalf("create index meta: %v", err)
	}
	if idx == nil || idx.Name == "" {
		t.Fatalf("missing index")
	}

	tenants, err := svc.ListTenants(ctx)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(tenants.Tenants) < 1 {
		t.Fatalf("expected default tenant")
	}
}

func TestVirtualRecordsMultiSelectArrayFilter(t *testing.T) {
	svc, cleanup := testutil.SetupIntegrationVR(t)
	defer cleanup()
	ctx := testutil.CtxVR()

	multiName := testutil.UniqueName("multi_select")
	if _, err := svc.CreateColumnType(ctx, &catalog.ColumnTypeDef{
		Name:  multiName,
		Label: "Multi Select",
		Spec:  &columntype.ColumnTypeSpec{PgType: "text", Array: true},
	}); err != nil {
		t.Fatalf("create columnType: %v", err)
	}

	table := testutil.UniqueName("vr_tags")
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: table}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: table, Name: "tags", TypeId: multiName, Position: 1,
	}); err != nil {
		t.Fatalf("add column: %v", err)
	}

	tags := []any{"urgent", "ops"}
	if _, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: table,
		Cells:     map[string]*shared.Value{"tags": shared.JsonValue(tags)},
	}); err != nil {
		t.Fatalf("create row: %v", err)
	}

	q, err := svc.QueryRows(ctx, &data.QueryRowsRequest{
		TableName: table,
		Filter: map[string]any{
			"type": "ARRAY_HAS",
			"attr": "tags",
			"val":  "urgent",
		},
	})
	if err != nil {
		t.Fatalf("ARRAY_HAS query: %v", err)
	}
	if len(q.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(q.Rows))
	}

	miss, err := svc.QueryRows(ctx, &data.QueryRowsRequest{
		TableName: table,
		Filter: map[string]any{
			"type": "ARRAY_HAS",
			"attr": "tags",
			"val":  "missing",
		},
	})
	if err != nil {
		t.Fatalf("ARRAY_HAS miss: %v", err)
	}
	if len(miss.Rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(miss.Rows))
	}
}
