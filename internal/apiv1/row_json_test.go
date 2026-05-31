package apiv1

import (
	"encoding/json"
	"testing"
)

func TestRow_MarshalJSON_flat(t *testing.T) {
	row := Row{
		Id: "1",
		Cells: map[string]*Value{
			"name": StringValue("客户1"),
		},
	}
	raw, err := json.Marshal(row)
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
	if req.Cells["name"] == nil || ValueString(req.Cells["name"]) != "客户1" {
		t.Fatalf("name cell=%v", req.Cells["name"])
	}
}

func TestRow_UnmarshalJSON_legacyCells(t *testing.T) {
	var row Row
	raw := `{"id":"1","cells":{"name":{"stringValue":"客户1"}}}`
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatal(err)
	}
	if row.Id != "1" || ValueString(row.Cells["name"]) != "客户1" {
		t.Fatalf("row=%+v", row)
	}
}
