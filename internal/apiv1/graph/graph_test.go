package graph_test

import (
	"encoding/json"
	"testing"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/graph"
	"github.com/solat/lowcode-database/internal/apiv1/row"
)

func TestSaveGraphRequest_Classify(t *testing.T) {
	raw := `{
		"id": "order-1",
		"amount": 100,
		"items": [
			{ "id": "item-1", "qty": 2, "goodsId": "g1" },
			{ "qty": 1, "goodsId": "g2" }
		],
		"_sync": { "items": "replace" }
	}`
	var req graph.SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	many := map[string]struct{}{"items": {}}
	if err := req.ClassifySaveGraphFields(many, map[string]struct{}{}); err != nil {
		t.Fatal(err)
	}
	items := req.ManyRelationships["items"]
	if items == nil || len(items.Data) != 2 {
		t.Fatalf("items=%+v", items)
	}
	if !items.Sync.ReplaceMissing() {
		t.Fatal("expected sync replace from _sync")
	}
}

func TestSaveGraphRequest_createOrderShape(t *testing.T) {
	raw := `{
		"orderRemark": "rush",
		"supply": { "code": "SUP-1" },
		"items": [ { "qty": 2, "goods_name": "Apple" } ]
	}`
	var req graph.SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.ClassifySaveGraphFields(
		map[string]struct{}{"items": {}},
		map[string]struct{}{"supply": {}},
	); err != nil {
		t.Fatal(err)
	}
	if apiv1.ValueString(req.RootCells["orderRemark"]) != "rush" {
		t.Fatalf("orderRemark=%v", req.RootCells["orderRemark"])
	}
	if req.OneRelationships["supply"] == nil {
		t.Fatal("missing supply")
	}
	if len(req.ManyRelationships["items"].Data) != 1 {
		t.Fatal("expected one item")
	}
}

func TestSaveGraphRequest_oneRelationship(t *testing.T) {
	raw := `{
		"amount": 100,
		"warehouse": { "name": "华东仓" }
	}`
	var req graph.SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	one := map[string]struct{}{"warehouse": {}}
	if err := req.ClassifySaveGraphFields(map[string]struct{}{}, one); err != nil {
		t.Fatal(err)
	}
	wh := req.OneRelationships["warehouse"]
	if wh == nil || apiv1.ValueString(wh.Cells["name"]) != "华东仓" {
		t.Fatalf("warehouse=%+v", wh)
	}
}

func TestSaveGraphRequest_legacyRowsCompat(t *testing.T) {
	raw := `{
		"items": {
			"rows": [{ "qty": 1 }],
			"deleteMissing": true
		}
	}`
	var req graph.SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.ClassifySaveGraphFields(map[string]struct{}{"items": {}}, nil); err != nil {
		t.Fatal(err)
	}
	items := req.ManyRelationships["items"]
	if items == nil || !items.Sync.ReplaceMissing() {
		t.Fatalf("items=%+v", items)
	}
}

func TestSaveGraphRequest_unknownFieldAsRootCell(t *testing.T) {
	raw := `{"name":"Acme"}`
	var req graph.SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.ClassifySaveGraphFields(map[string]struct{}{}, map[string]struct{}{}); err != nil {
		t.Fatal(err)
	}
	if apiv1.ValueString(req.RootCells["name"]) != "Acme" {
		t.Fatalf("name=%v", req.RootCells["name"])
	}
}

func TestParseSaveGraphManyInput_requiresData(t *testing.T) {
	_, err := graph.ParseSaveGraphManyInput([]byte(`{"sync":"replace"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSaveGraphManyInput_arrayShorthand(t *testing.T) {
	in, err := graph.ParseSaveGraphManyInput([]byte(`[{ "qty": 1 }]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Data) != 1 || in.Sync != graph.SaveGraphSyncMerge {
		t.Fatalf("input=%+v", in)
	}
}

func TestParseSaveGraphOneInput_rejectsArray(t *testing.T) {
	_, err := graph.ParseSaveGraphOneInput([]byte(`[{ "name": "x" }]`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifySaveGraphRowPayload_nestedOne(t *testing.T) {
	payload, err := graph.ClassifySaveGraphRowPayload(
		[]byte(`{ "qty": 2, "goods": { "name": "Apple" } }`),
		map[string]struct{}{},
		map[string]struct{}{"goods": {}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if apiv1.ValueNumber(payload.Cells["qty"]) != 2 {
		t.Fatalf("qty=%v", payload.Cells["qty"])
	}
	goods := payload.OneRelationships["goods"]
	if goods == nil || apiv1.ValueString(goods.Cells["name"]) != "Apple" {
		t.Fatalf("goods=%+v", goods)
	}
}

func TestBuildSaveGraphEcho(t *testing.T) {
	req := &graph.SaveGraphRequest{
		Id: "order-1",
		Fields: map[string]json.RawMessage{
			"amount":    json.RawMessage(`100`),
			"warehouse": json.RawMessage(`{"name":"华东仓"}`),
			"items":     json.RawMessage(`[{"qty":2,"goods":{"name":"Apple"}}]`),
		},
		RootCells: map[string]*apiv1.Value{
			"amount": apiv1.NumberValue(100),
		},
	}
	out := &graph.SaveGraphSaveOutcome{
		RootID: "order-1",
		RootCells: map[string]*apiv1.Value{
			"amount":       apiv1.NumberValue(100),
			"warehouse_id": apiv1.StringValue("wh-1"),
		},
		One: map[string]*row.Row{
			"warehouse": {Id: "wh-1", Cells: map[string]*apiv1.Value{"name": apiv1.StringValue("华东仓")}},
		},
		Many: map[string][]*graph.SaveGraphChildSaveOutcome{
			"items": {{
				Row: &row.Row{
					Id: "item-1",
					Cells: map[string]*apiv1.Value{
						"qty":      apiv1.NumberValue(2),
						"goods_id": apiv1.StringValue("g-1"),
					},
				},
				OneRelationships: map[string]*row.Row{
					"goods": {Id: "g-1", Cells: map[string]*apiv1.Value{"name": apiv1.StringValue("Apple")}},
				},
			}},
		},
		ManyLinkColumns: map[string]string{"items": "order_id"},
	}
	echo := graph.BuildSaveGraphEcho(req, out)
	if echo["id"] != "order-1" {
		t.Fatalf("id=%v", echo["id"])
	}
	wh, ok := echo["warehouse"].(map[string]any)
	if !ok || wh["id"] != "wh-1" || wh["name"] != "华东仓" {
		t.Fatalf("warehouse=%+v", echo["warehouse"])
	}
	items, ok := echo["items"].([]map[string]any)
	if !ok {
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
