package apiv1

import (
	"encoding/json"
	"testing"
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
	var req SaveGraphRequest
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
	var req SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.ClassifySaveGraphFields(
		map[string]struct{}{"items": {}},
		map[string]struct{}{"supply": {}},
	); err != nil {
		t.Fatal(err)
	}
	if ValueString(req.RootCells["orderRemark"]) != "rush" {
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
	var req SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	one := map[string]struct{}{"warehouse": {}}
	if err := req.ClassifySaveGraphFields(map[string]struct{}{}, one); err != nil {
		t.Fatal(err)
	}
	wh := req.OneRelationships["warehouse"]
	if wh == nil || ValueString(wh.Cells["name"]) != "华东仓" {
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
	var req SaveGraphRequest
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
	var req SaveGraphRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.ClassifySaveGraphFields(map[string]struct{}{}, map[string]struct{}{}); err != nil {
		t.Fatal(err)
	}
	if ValueString(req.RootCells["name"]) != "Acme" {
		t.Fatalf("name=%v", req.RootCells["name"])
	}
}

func TestParseSaveGraphManyInput_requiresData(t *testing.T) {
	_, err := ParseSaveGraphManyInput([]byte(`{"sync":"replace"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSaveGraphManyInput_arrayShorthand(t *testing.T) {
	in, err := ParseSaveGraphManyInput([]byte(`[{ "qty": 1 }]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Data) != 1 || in.Sync != SaveGraphSyncMerge {
		t.Fatalf("input=%+v", in)
	}
}

func TestParseSaveGraphOneInput_rejectsArray(t *testing.T) {
	_, err := ParseSaveGraphOneInput([]byte(`[{ "name": "x" }]`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifySaveGraphRowPayload_nestedOne(t *testing.T) {
	payload, err := ClassifySaveGraphRowPayload(
		[]byte(`{ "qty": 2, "goods": { "name": "Apple" } }`),
		map[string]struct{}{},
		map[string]struct{}{"goods": {}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ValueNumber(payload.Cells["qty"]) != 2 {
		t.Fatalf("qty=%v", payload.Cells["qty"])
	}
	goods := payload.OneRelationships["goods"]
	if goods == nil || ValueString(goods.Cells["name"]) != "Apple" {
		t.Fatalf("goods=%+v", goods)
	}
}
