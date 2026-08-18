package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

// TestBidirectionalLinkOrderItems mirrors Teable two-way links:
// order.order_items (many) ↔ order_items.order (one). Creating an item
// linked to an order makes the item visible from the order side.
func TestBidirectionalLinkOrderItems(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	orderTable := testutil.UniqueName("order")
	itemsTable := testutil.UniqueName("order_items")

	if _, err := svc.CreateTable(ctx, &schema.Table{Name: orderTable, Label: "Orders"}); err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: itemsTable, Label: "Order items"}); err != nil {
		t.Fatalf("create order_items: %v", err)
	}

	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orderTable, Name: "title", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatalf("order.title: %v", err)
	}
	if _, err := svc.AddColumn(ctx, &schema.Column{
		TableName: itemsTable, Name: "sku", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatalf("order_items.sku: %v", err)
	}

	linkCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orderTable, Name: "order_items", TypeId: "link", Position: 2,
		Config: map[string]any{
			"target_table_name":   itemsTable,
			"cardinality":         "many",
			"bidirectional":       true,
			"inverse_field_name":  "order",
		},
	})
	if err != nil {
		t.Fatalf("add bidirectional link: %v", err)
	}
	if !shared.CfgBool(linkCol.Config, "bidirectional") {
		t.Fatal("expected bidirectional on order.order_items")
	}
	if shared.CfgString(linkCol.Config, "inverse_field_name") != "order" {
		t.Fatalf("inverse_field_name=%q", shared.CfgString(linkCol.Config, "inverse_field_name"))
	}

	itemCols, err := svc.ListColumns(ctx, itemsTable)
	if err != nil {
		t.Fatalf("list order_items columns: %v", err)
	}
	var inverse *schema.Column
	for _, c := range itemCols {
		if c.Name == "order" {
			inverse = c
			break
		}
	}
	if inverse == nil {
		t.Fatal("expected auto-created inverse link order_items.order")
	}
	if shared.CfgString(inverse.Config, "cardinality") != "one" {
		t.Fatalf("inverse cardinality=%q want one", shared.CfgString(inverse.Config, "cardinality"))
	}
	if shared.CfgString(inverse.Config, "target_table_name") != orderTable {
		t.Fatalf("inverse target=%q", shared.CfgString(inverse.Config, "target_table_name"))
	}

	orderRow, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: orderTable,
		Cells: map[string]*shared.Value{
			"title": shared.StringValue("PO-1001"),
		},
	})
	if err != nil {
		t.Fatalf("create order row: %v", err)
	}

	itemRow, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: itemsTable,
		Cells: map[string]*shared.Value{
			"sku":   shared.StringValue("SKU-A"),
			"order": shared.JsonValue([]string{orderRow.Row.Id}),
		},
	})
	if err != nil {
		t.Fatalf("create order_items row with order link: %v", err)
	}

	gotOrder, err := svc.GetRow(ctx, &data.GetRowRequest{TableName: orderTable, RowId: orderRow.Row.Id})
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	ids := shared.ValueToNative(gotOrder.Row.Cells["order_items"])
	got := asStringIDs(ids)
	if len(got) != 1 || got[0] != itemRow.Row.Id {
		t.Fatalf("order.order_items=%v want [%s]", got, itemRow.Row.Id)
	}

	gotItem, err := svc.GetRow(ctx, &data.GetRowRequest{TableName: itemsTable, RowId: itemRow.Row.Id})
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	orderIDs := asStringIDs(shared.ValueToNative(gotItem.Row.Cells["order"]))
	if len(orderIDs) != 1 || orderIDs[0] != orderRow.Row.Id {
		t.Fatalf("order_items.order=%v want [%s]", orderIDs, orderRow.Row.Id)
	}

	// Writing from the many side should also sync.
	item2, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: itemsTable,
		Cells:     map[string]*shared.Value{"sku": shared.StringValue("SKU-B")},
	})
	if err != nil {
		t.Fatalf("create second item: %v", err)
	}
	_, err = svc.UpdateRow(ctx, &data.UpdateRowRequest{
		TableName: orderTable,
		RowId:     orderRow.Row.Id,
		Cells: map[string]*shared.Value{
			"order_items": shared.JsonValue([]string{itemRow.Row.Id, item2.Row.Id}),
		},
	})
	if err != nil {
		t.Fatalf("update order.order_items: %v", err)
	}
	gotItem2, err := svc.GetRow(ctx, &data.GetRowRequest{TableName: itemsTable, RowId: item2.Row.Id})
	if err != nil {
		t.Fatalf("get item2: %v", err)
	}
	orderIDs2 := asStringIDs(shared.ValueToNative(gotItem2.Row.Cells["order"]))
	if len(orderIDs2) != 1 || orderIDs2[0] != orderRow.Row.Id {
		t.Fatalf("item2.order=%v want [%s]", orderIDs2, orderRow.Row.Id)
	}
}

func asStringIDs(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, stringifyID(x))
		}
		return out
	default:
		if v == nil {
			return nil
		}
		return []string{stringifyID(v)}
	}
}

func stringifyID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
