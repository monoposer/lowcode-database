package apiv1

import (
	"encoding/json"
	"testing"
)

func TestBuildSaveGraphEcho(t *testing.T) {
	req := &SaveGraphRequest{
		Id: "order-1",
		Fields: map[string]json.RawMessage{
			"amount":    json.RawMessage(`100`),
			"warehouse": json.RawMessage(`{"name":"华东仓"}`),
			"items":     json.RawMessage(`[{"qty":2,"goods":{"name":"Apple"}}]`),
		},
		RootCells: map[string]*Value{
			"amount": NumberValue(100),
		},
	}
	out := &SaveGraphSaveOutcome{
		RootID: "order-1",
		RootCells: map[string]*Value{
			"amount":        NumberValue(100),
			"warehouse_id":  StringValue("wh-1"),
		},
		One: map[string]*Row{
			"warehouse": {Id: "wh-1", Cells: map[string]*Value{"name": StringValue("华东仓")}},
		},
		Many: map[string][]*SaveGraphChildSaveOutcome{
			"items": {{
				Row: &Row{
					Id: "item-1",
					Cells: map[string]*Value{
						"qty":      NumberValue(2),
						"goods_id": StringValue("g-1"),
					},
				},
				OneRelationships: map[string]*Row{
					"goods": {Id: "g-1", Cells: map[string]*Value{"name": StringValue("Apple")}},
				},
			}},
		},
		ManyLinkColumns: map[string]string{"items": "order_id"},
	}
	echo := BuildSaveGraphEcho(req, out)
	if echo["id"] != "order-1" {
		t.Fatalf("id=%v", echo["id"])
	}
	wh, ok := echo["warehouse"].(map[string]any)
	if !ok || wh["id"] != "wh-1" || wh["name"] != "华东仓" {
		t.Fatalf("warehouse=%+v", echo["warehouse"])
	}
	items, ok := echo["items"].([]map[string]any)
	if !ok {
		// json may decode as []any
		if arr, ok2 := echo["items"].([]any); ok2 && len(arr) > 0 {
			if m, ok3 := arr[0].(map[string]any); ok3 {
				if m["id"] != "item-1" {
					t.Fatalf("item id=%v", m["id"])
				}
				goods, _ := m["goods"].(map[string]any)
				if goods["id"] != "g-1" {
					t.Fatalf("goods=%+v", goods)
				}
				return
			}
		}
		t.Fatalf("items=%+v", echo["items"])
	}
	if items[0]["id"] != "item-1" {
		t.Fatalf("item=%+v", items[0])
	}
}
