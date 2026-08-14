package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

func TestIntegrationSharedDSNIndexTenantIsolation(t *testing.T) {
	svc, cleanup := testutil.SetupIntegrationSharedDB(t)
	defer cleanup()

	tableName := testutil.UniqueName("orders")
	indexLogical := "status_idx"

	for i, tid := range []string{testutil.SharedTenantA(), testutil.SharedTenantB()} {
		ctx := testutil.CtxTenant(tid)
		if _, err := svc.CreateTable(ctx, &schema.Table{Name: tableName}); err != nil {
			t.Fatalf("tenant %d create table: %v", i, err)
		}
		col, err := svc.AddColumn(ctx, &schema.Column{
			TableName: tableName, Name: "status", TypeId: "text", Position: 1,
		})
		if err != nil {
			t.Fatalf("tenant %d add column: %v", i, err)
		}
		_, err = svc.CreateIndex(ctx, &catalog.Index{
			TableName: tableName, Name: indexLogical, ColumnIds: []string{col.Id},
		})
		if err != nil {
			t.Fatalf("tenant %d create index: %v", i, err)
		}
	}

	ctxA := testutil.CtxTenant(testutil.SharedTenantA())
	got, err := svc.GetIndex(ctxA, tableName, indexLogical)
	if err != nil {
		t.Fatalf("tenant A get index: %v", err)
	}
	if got.TableName != tableName || got.Name != indexLogical {
		t.Fatalf("tenant A index: %+v", got)
	}

	ctxB := testutil.CtxTenant(testutil.SharedTenantB())
	if err := svc.DeleteIndex(ctxB, tableName, indexLogical); err != nil {
		t.Fatalf("tenant B delete index: %v", err)
	}

	listA, err := svc.ListIndexes(ctxA, tableName)
	if err != nil {
		t.Fatalf("tenant A list indexes: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("tenant A should still have index, got %d", len(listA))
	}

	listB, err := svc.ListIndexes(ctxB, tableName)
	if err != nil {
		t.Fatalf("tenant B list indexes: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("tenant B should have no indexes, got %d", len(listB))
	}
}

func TestIntegrationCreateIndex(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	tableName := testutil.UniqueName("import_tbl")
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: tableName}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	col, err := svc.AddColumn(ctx, &schema.Column{
		TableName: tableName, Name: "name", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatalf("add column: %v", err)
	}
	if _, err := svc.CreateIndex(ctx, &catalog.Index{
		TableName: tableName, Name: "name_idx", ColumnIds: []string{col.Id},
	}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	idxList, err := svc.ListIndexes(ctx, tableName)
	if err != nil || len(idxList) != 1 {
		t.Fatalf("list indexes: %v len=%d", err, len(idxList))
	}
	if idxList[0].Name != "name_idx" {
		t.Fatalf("index name: %+v", idxList[0])
	}
}
