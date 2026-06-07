package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

func TestVirtualRecordsCRUD(t *testing.T) {
	svc, cleanup := testutil.SetupIntegrationVR(t)
	defer cleanup()
	ctx := testutil.CtxVR()

	table := testutil.UniqueName("vr_orders")
	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: table}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	titleCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: table, Name: "title", TypeId: "text", Position: 2,
		Config: map[string]any{"enable_fulltext": true, "need_index": true},
	})
	if err != nil {
		t.Fatalf("add column: %v", err)
	}
	_ = titleCol

	str := "hello virtual records"
	created, err := svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: table,
		Cells:   map[string]*apiv1.Value{"title": {StringValue: &str}},
	})
	if err != nil {
		t.Fatalf("create row: %v", err)
	}
	if created.Row == nil || created.Row.Id == "" {
		t.Fatalf("missing row id")
	}

	q, err := svc.QueryRows(ctx, &row.QueryRowsRequest{
		TableId: table,
		Filter:  map[string]any{"field": "title", "op": "eq", "value": str},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(q.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(q.Rows))
	}

	search, err := svc.SearchRows(ctx, &row.SearchRowsRequest{TableId: table, Query: "virtual"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(search.Rows) < 1 {
		t.Fatalf("fts search expected hits, got %d", len(search.Rows))
	}

	idx, err := svc.CreateIndex(ctx, &apiv1schema.CreateIndexRequest{
		TableId: table, Name: "by_title", ColumnIds: []string{titleCol.Column.Id},
	})
	if err != nil {
		t.Fatalf("create index meta: %v", err)
	}
	if idx.Index == nil || idx.Index.Name == "" {
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
