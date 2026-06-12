package row_test

import (
	"encoding/json"
	"testing"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/row"
)

func TestRow_MarshalJSON_flat(t *testing.T) {
	r := row.Row{
		Id: "1",
		Cells: map[string]*apiv1.Value{
			"name": apiv1.StringValue("客户1"),
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
	var req row.CreateRowRequest
	if err := json.Unmarshal([]byte(`{"name":"客户1","status":"active"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Cells["name"] == nil || apiv1.ValueString(req.Cells["name"]) != "客户1" {
		t.Fatalf("name cell=%v", req.Cells["name"])
	}
}

func TestRow_UnmarshalJSON_legacyCells(t *testing.T) {
	var r row.Row
	raw := `{"id":"1","cells":{"name":{"stringValue":"客户1"}}}`
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	if r.Id != "1" || apiv1.ValueString(r.Cells["name"]) != "客户1" {
		t.Fatalf("row=%+v", r)
	}
}
