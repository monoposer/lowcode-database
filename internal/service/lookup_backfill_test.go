package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/internal/testutil"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

// Adding a lookup after rows exist must backfill list cache; changing a link
// must refresh lookup on the child and rollup on old/new parents.
func TestLookupRollupBackfillOnAddAndLinkWrite(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	orders := testutil.UniqueName("orders")
	goods := testutil.UniqueName("goods")
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: orders, Label: "Orders"}); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: goods, Label: "Goods"}); err != nil {
		t.Fatalf("create goods: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orders, Name: "customer_name", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatalf("orders.customer_name: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: goods, Name: "description", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatalf("goods.description: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: goods, Name: "orderId", TypeId: "link", Position: 2,
		Config: map[string]any{
			"target_table_name":  orders,
			"cardinality":        "one",
			"bidirectional":      true,
			"inverse_field_name": "goods",
		},
	}); err != nil {
		t.Fatalf("goods.orderId: %v", err)
	}

	order1, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: orders,
		Cells:     map[string]*shared.Value{"customer_name": shared.StringValue("BBB")},
	})
	if err != nil {
		t.Fatalf("create order1: %v", err)
	}
	g1, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: goods,
		Cells: map[string]*shared.Value{
			"description": shared.StringValue("新"),
			"orderId":     shared.StringValue(order1.Row.Id),
		},
	})
	if err != nil {
		t.Fatalf("create goods1: %v", err)
	}
	g2, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: goods,
		Cells: map[string]*shared.Value{
			"description": shared.StringValue("商品"),
			"orderId":     shared.StringValue(order1.Row.Id),
		},
	})
	if err != nil {
		t.Fatalf("create goods2: %v", err)
	}

	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: goods, Name: "order_c_name", TypeId: "lookup", Position: 3,
		Config: map[string]any{
			"relation_column_id": "orderId",
			"target_column_id":   "customer_name",
		},
	}); err != nil {
		t.Fatalf("add lookup: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orders, Name: "item_count", TypeId: "rollup", Position: 2,
		Config: map[string]any{
			"relation_column_id": "goods",
			"aggregation":        "count",
		},
	}); err != nil {
		t.Fatalf("add rollup: %v", err)
	}
	drainCalc(t, svc, ctx, g1.Row.Id, g2.Row.Id, order1.Row.Id)

	if got := nativeCell(queryRow(t, svc, ctx, goods, g1.Row.Id), "order_c_name"); fmt.Sprint(got) != "BBB" {
		t.Fatalf("goods1 lookup after add-column = %v want BBB", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, goods, g2.Row.Id), "order_c_name"); fmt.Sprint(got) != "BBB" {
		t.Fatalf("goods2 lookup after add-column = %v want BBB", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, orders, order1.Row.Id), "item_count"); !numEq(got, 2) {
		t.Fatalf("order1 rollup after add-column = %v want 2", got)
	}

	g3, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: goods,
		Cells: map[string]*shared.Value{
			"description": shared.StringValue("新商品"),
			"orderId":     shared.StringValue(order1.Row.Id),
		},
	})
	if err != nil {
		t.Fatalf("create goods3: %v", err)
	}
	drainCalc(t, svc, ctx, g3.Row.Id, order1.Row.Id)
	if got := nativeCell(queryRow(t, svc, ctx, goods, g3.Row.Id), "order_c_name"); fmt.Sprint(got) != "BBB" {
		t.Fatalf("goods3 lookup after create = %v want BBB", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, orders, order1.Row.Id), "item_count"); !numEq(got, 3) {
		t.Fatalf("order1 rollup after create = %v want 3", got)
	}

	order2, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: orders,
		Cells:     map[string]*shared.Value{"customer_name": shared.StringValue("AAA")},
	})
	if err != nil {
		t.Fatalf("create order2: %v", err)
	}
	if _, err := svc.UpdateRow(ctx, &data.UpdateRowRequest{
		TableName: goods,
		RowId:     g2.Row.Id,
		Cells:     map[string]*shared.Value{"orderId": shared.StringValue(order2.Row.Id)},
	}); err != nil {
		t.Fatalf("relink goods2: %v", err)
	}
	drainCalc(t, svc, ctx, g1.Row.Id, g2.Row.Id, order1.Row.Id, order2.Row.Id)
	if got := nativeCell(queryRow(t, svc, ctx, goods, g2.Row.Id), "order_c_name"); fmt.Sprint(got) != "AAA" {
		t.Fatalf("goods2 lookup after relink = %v want AAA", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, goods, g1.Row.Id), "order_c_name"); fmt.Sprint(got) != "BBB" {
		t.Fatalf("goods1 lookup after relink = %v want BBB", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, orders, order1.Row.Id), "item_count"); !numEq(got, 2) {
		t.Fatalf("order1 rollup after relink = %v want 2", got)
	}
	if got := nativeCell(queryRow(t, svc, ctx, orders, order2.Row.Id), "item_count"); !numEq(got, 1) {
		t.Fatalf("order2 rollup after relink = %v want 1", got)
	}
}

func drainCalc(t *testing.T, svc *service.LowcodeService, ctx context.Context, recordIDs ...string) {
	t.Helper()
	ctx, _, err := svc.Schema.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		t.Fatalf("attach data tables: %v", err)
	}
	pool, err := svc.Schema.B.Tenants.DataPool(ctx)
	if err != nil {
		t.Fatalf("data pool: %v", err)
	}
	meta := svc.Schema.B.Tenants.MetaPool()
	w := calc.NewWorker(calc.WorkerConfig{Tenants: svc.Schema.B.Tenants, Batch: 200})
	q := postgres.TablesFromContext(ctx).QCalcQueue()
	for i := 0; i < 40; i++ {
		if err := w.Drain(ctx, meta, pool); err != nil {
			t.Fatalf("drain calc: %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::int FROM %s WHERE record_id = ANY($1) AND status IN (0, 1)`, q), recordIDs).Scan(&n); err != nil {
			t.Fatalf("active calc: %v", err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatalf("calc queue did not drain for %v", recordIDs)
}

func queryRow(t *testing.T, svc *service.LowcodeService, ctx context.Context, table, id string) *data.Row {
	t.Helper()
	q, err := svc.QueryRows(ctx, &data.QueryRowsRequest{
		TableName: table,
		Filter:    map[string]any{"type": "EQ", "attr": "id", "val": id},
	})
	if err != nil {
		t.Fatalf("query %s %s: %v", table, id, err)
	}
	if len(q.Rows) != 1 {
		t.Fatalf("query %s %s: want 1 row, got %d", table, id, len(q.Rows))
	}
	return q.Rows[0]
}

func nativeCell(row *data.Row, name string) any {
	if row == nil {
		return nil
	}
	return shared.ValueToNative(row.Cells[name])
}

func numEq(v any, want float64) bool {
	var f float64
	switch n := v.(type) {
	case float64:
		f = n
	case float32:
		f = float64(n)
	case int:
		f = float64(n)
	case int32:
		f = float64(n)
	case int64:
		f = float64(n)
	default:
		if _, err := fmt.Sscan(fmt.Sprint(v), &f); err != nil {
			return false
		}
	}
	return f == want
}
