package service_test

import (
	"testing"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

func TestIntegrationSharedDBIndexTenantIsolation(t *testing.T) {
	svc, cleanup := testutil.SetupIntegrationSharedDB(t)
	defer cleanup()

	tableName := testutil.UniqueName("orders")
	indexLogical := "status_idx"

	for i, tid := range []string{testutil.SharedTenantA(), testutil.SharedTenantB()} {
		ctx := testutil.CtxTenant(tid)
		if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: tableName}); err != nil {
			t.Fatalf("tenant %d create table: %v", i, err)
		}
		col, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
			TableId: tableName, Name: "status", TypeId: "text", Position: 1,
		})
		if err != nil {
			t.Fatalf("tenant %d add column: %v", i, err)
		}
		_, err = svc.CreateIndex(ctx, &apiv1schema.CreateIndexRequest{
			TableId: tableName, Name: indexLogical, ColumnIds: []string{col.Column.Id},
		})
		if err != nil {
			t.Fatalf("tenant %d create index: %v", i, err)
		}
	}

	ctxA := testutil.CtxTenant(testutil.SharedTenantA())
	got, err := svc.GetIndex(ctxA, &apiv1schema.GetIndexRequest{Id: indexLogical, TableId: tableName})
	if err != nil {
		t.Fatalf("tenant A get index: %v", err)
	}
	if got.Index.TableId != tableName || got.Index.Name != indexLogical {
		t.Fatalf("tenant A index: %+v", got.Index)
	}

	ctxB := testutil.CtxTenant(testutil.SharedTenantB())
	if _, err := svc.DeleteIndex(ctxB, &apiv1schema.DeleteIndexRequest{Id: indexLogical, TableId: tableName}); err != nil {
		t.Fatalf("tenant B delete index: %v", err)
	}

	listA, err := svc.ListIndexes(ctxA, &apiv1schema.ListIndexesRequest{TableId: tableName})
	if err != nil {
		t.Fatalf("tenant A list indexes: %v", err)
	}
	if len(listA.Indexes) != 1 {
		t.Fatalf("tenant A should still have index, got %d", len(listA.Indexes))
	}

	listB, err := svc.ListIndexes(ctxB, &apiv1schema.ListIndexesRequest{TableId: tableName})
	if err != nil {
		t.Fatalf("tenant B list indexes: %v", err)
	}
	if len(listB.Indexes) != 0 {
		t.Fatalf("tenant B should have no indexes, got %d", len(listB.Indexes))
	}
}

func TestIntegrationCreateIndex(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	tableName := testutil.UniqueName("import_tbl")
	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: tableName}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	col, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: tableName, Name: "name", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatalf("add column: %v", err)
	}
	if _, err := svc.CreateIndex(ctx, &apiv1schema.CreateIndexRequest{
		TableId: tableName, Name: "name_idx", ColumnIds: []string{col.Column.Id},
	}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	idxList, err := svc.ListIndexes(ctx, &apiv1schema.ListIndexesRequest{TableId: tableName})
	if err != nil || len(idxList.Indexes) != 1 {
		t.Fatalf("list indexes: %v len=%d", err, len(idxList.Indexes))
	}
	if idxList.Indexes[0].Name != "name_idx" {
		t.Fatalf("index name: %+v", idxList.Indexes[0])
	}
}
