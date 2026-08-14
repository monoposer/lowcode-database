package data

import (
	"encoding/json"
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestRow_MarshalJSON_flat(t *testing.T) {
	r := Row{
		Id: "1",
		Cells: map[string]*shared.Value{
			"name": shared.StringValue("客户1"),
		},
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["id"] != "1" {
		t.Fatalf("id=%v", m["id"])
	}
	if m["name"] != "客户1" {
		t.Fatalf("name=%v", m["name"])
	}
	if _, ok := m["cells"]; ok {
		t.Fatal("must not contain cells key")
	}
}

func TestCreateRowRequest_UnmarshalJSON_flat(t *testing.T) {
	var req CreateRowRequest
	if err := json.Unmarshal([]byte(`{"name":"客户1","status":"active"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Cells["name"] == nil || shared.ValueString(req.Cells["name"]) != "客户1" {
		t.Fatalf("name cell=%v", req.Cells["name"])
	}
}

func TestRow_UnmarshalJSON_cellsObject(t *testing.T) {
	var r Row
	raw := `{"id":"1","cells":{"name":{"stringValue":"客户1"}}}`
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	if r.Id != "1" || shared.ValueString(r.Cells["name"]) != "客户1" {
		t.Fatalf("row=%+v", r)
	}
}
