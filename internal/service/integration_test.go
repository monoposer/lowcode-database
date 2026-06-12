package service_test

import (
	"encoding/json"
	"fmt"
	"github.com/solat/lowcode-database/internal/apiv1"

	"github.com/solat/lowcode-database/internal/apiv1/datasource"

	"github.com/solat/lowcode-database/internal/apiv1/graph"

	"github.com/solat/lowcode-database/internal/apiv1/platform"

	"github.com/solat/lowcode-database/internal/apiv1/row"

	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"

	"github.com/solat/lowcode-database/internal/testutil"
	"testing"
)

func saveGraphItems(t *testing.T, resp graph.SaveGraphResponse) []map[string]any {
	t.Helper()
	switch v := resp["items"].(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, it := range v {
			m, ok := it.(map[string]any)
			if !ok {
				t.Fatalf("item=%+v", it)
			}
			out = append(out, m)
		}
		return out
	default:
		t.Fatalf("items=%+v", resp["items"])
		return nil
	}
}

func saveGraphNested(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s=%+v", key, m[key])
	}
	return v
}

func TestIntegrationFullWorkflow(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	// --- tables ---
	vendorTable := testutil.UniqueName("vendor")
	orderTable := testutil.UniqueName("order")

	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: vendorTable}); err != nil {
		t.Fatalf("create vendor table: %v", err)
	}
	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: orderTable}); err != nil {
		t.Fatalf("create order table: %v", err)
	}

	// --- columns: scalar types ---
	nameCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "name", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatalf("add text column: %v", err)
	}
	scoreCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "score", TypeId: "double", Position: 2,
	})
	if err != nil {
		t.Fatalf("add double column: %v", err)
	}
	activeCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "active", TypeId: "bool", Position: 3,
	})
	if err != nil {
		t.Fatalf("add bool column: %v", err)
	}
	metaCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "meta", TypeId: "jsonb", Position: 4,
	})
	if err != nil {
		t.Fatalf("add jsonb column: %v", err)
	}
	_ = nameCol
	_ = scoreCol
	_ = activeCol
	_ = metaCol

	// int8 id column on vendor for FK demo
	vendorIDCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "legacy_id", TypeId: "int8", Position: 5,
	})
	if err != nil {
		t.Fatalf("add int8 column: %v", err)
	}

	amountCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "amount", TypeId: "precision", Position: 1,
	})
	if err != nil {
		t.Fatalf("add precision column: %v", err)
	}

	// relation_fk on order -> vendor legacy_id
	fkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "vendor_ref", TypeId: "relation_fk", Position: 2,
		Config: map[string]any{
			"target_table_id":  vendorTable,
			"target_column_id": vendorIDCol.Column.Id,
		},
	})
	if err != nil {
		t.Fatalf("add relation_fk: %v", err)
	}

	// relationship many: orders linked by vendor uuid id
	linkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "vendor_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatalf("add uuid column: %v", err)
	}

	relCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "orders", TypeId: "relationship", Position: 6,
		Config: map[string]any{
			"target_table_id": orderTable,
			"link_column_id":  linkCol.Column.Id,
			"cardinality":     "many",
		},
	})
	if err != nil {
		t.Fatalf("add relationship: %v", err)
	}

	// formula on vendor
	formulaCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "double_score", TypeId: "formula", Position: 7,
		Config: map[string]any{"expression": "{{score}} * 2"},
	})
	if err != nil {
		t.Fatalf("add formula: %v", err)
	}

	// --- choice (PG ENUM) ---
	choiceName := testutil.UniqueName("status")
	choiceResp, err := svc.CreateChoice(ctx, &apiv1schema.CreateChoiceRequest{
		Name:  choiceName,
		Label: "Status",
		Values: []*apiv1schema.ChoiceItem{
			{Value: "active", Label: "Active"},
			{Value: "inactive", Label: "Inactive"},
		},
	})
	if err != nil {
		t.Fatalf("create choice: %v", err)
	}

	statusCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "status", TypeId: choiceName, Position: 8,
	})
	if err != nil {
		t.Fatalf("add enum column: %v", err)
	}
	_ = statusCol

	// --- relation registry ---
	_, err = svc.CreateRelation(ctx, &apiv1schema.CreateRelationRequest{
		Name:           testutil.UniqueName("order_vendor"),
		Kind:           "MANY_TO_ONE",
		SourceTableId:  orderTable,
		SourceColumnId: fkCol.Column.Name,
		TargetTableId:  vendorTable,
		TargetColumnId: vendorIDCol.Column.Name,
	})
	if err != nil {
		t.Fatalf("create relation: %v", err)
	}

	// --- rows ---
	vendorRow, err := svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: vendorTable,
		Cells: map[string]*apiv1.Value{
			nameCol.Column.Id:     apiv1.StringValue("Acme"),
			scoreCol.Column.Id:    apiv1.NumberValue(10),
			activeCol.Column.Id:   apiv1.BoolValue(true),
			metaCol.Column.Id:     apiv1.JsonValue(map[string]any{"tier": "gold"}),
			vendorIDCol.Column.Id: apiv1.NumberValue(1001),
		},
	})
	if err != nil {
		t.Fatalf("create vendor row: %v", err)
	}

	_, err = svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: orderTable,
		Cells: map[string]*apiv1.Value{
			amountCol.Column.Id: apiv1.NumberValue(99.5),
			fkCol.Column.Id:     apiv1.NumberValue(1001),
			linkCol.Column.Id:   apiv1.StringValue(vendorRow.Row.Id),
		},
	})
	if err != nil {
		t.Fatalf("create order row: %v", err)
	}

	// --- index ---
	_, err = svc.CreateIndex(ctx, &apiv1schema.CreateIndexRequest{
		TableId: vendorTable, Name: "idx_score", ColumnIds: []string{scoreCol.Column.Id},
	})
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	idxList, err := svc.ListIndexes(ctx, &apiv1schema.ListIndexesRequest{TableId: vendorTable})
	if err != nil || len(idxList.Indexes) == 0 {
		t.Fatalf("list indexes: %v len=%d", err, len(idxList.Indexes))
	}

	// --- data source (list / view definition) ---
	dsResp, err := svc.CreateDataSource(ctx, &datasource.CreateDataSourceRequest{
		Name:      testutil.UniqueName("active_vendors"),
		Label:     "Active Vendors",
		TableId:   vendorTable,
		ColumnIds: []string{nameCol.Column.Name, scoreCol.Column.Name, formulaCol.Column.Name},
		Filter: map[string]any{
			"type": "EQ", "attr": activeCol.Column.Name, "val": true,
		},
		Sort: []*apiv1.SortOrder{{Attribute: scoreCol.Column.Name, SortOrder: "DESC"}},
	})
	if err != nil {
		t.Fatalf("create data source: %v", err)
	}

	dsQuery, err := svc.QueryDataSource(ctx, &datasource.QueryDataSourceRequest{
		TableId:      vendorTable,
		DataSourceId: dsResp.DataSource.Name,
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("query data source: %v", err)
	}
	if len(dsQuery.Rows) == 0 {
		t.Fatal("data source query returned no rows")
	}
	if dsQuery.Rows[0].Cells[formulaCol.Column.Id] == nil {
		t.Fatal("formula column missing in data source query")
	}

	// --- query rows with filter ---
	qrows, err := svc.QueryRows(ctx, &row.QueryRowsRequest{
		TableId:  vendorTable,
		Filter:   map[string]any{"type": "EQ", "attr": nameCol.Column.Id, "val": "Acme"},
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	if len(qrows.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(qrows.Rows))
	}

	// --- ER diagram ---
	er, err := svc.GetERDiagram(ctx, &apiv1schema.GetERDiagramRequest{})
	if err != nil {
		t.Fatalf("er diagram: %v", err)
	}
	if len(er.Diagram.Nodes) < 2 {
		t.Fatalf("expected >=2 nodes, got %d", len(er.Diagram.Nodes))
	}
	if len(er.Diagram.Edges) == 0 {
		t.Fatal("expected edges in ER diagram")
	}

	// --- list tables / schema ---
	tables, err := svc.ListTables(ctx, &apiv1schema.ListTablesRequest{})
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	found := false
	for _, tbl := range tables.Tables {
		if tbl.Id == vendorTable {
			found = true
		}
	}
	if !found {
		t.Fatal("vendor table not in list")
	}

	schema, err := svc.GetTableSchema(ctx, &apiv1schema.GetTableSchemaRequest{TableId: vendorTable})
	if err != nil {
		t.Fatalf("get schema: %v", err)
	}
	if len(schema.Columns) < 5 {
		t.Fatalf("expected many columns, got %d", len(schema.Columns))
	}

	// --- choice resolve & replace ---
	items, err := svc.ResolveChoiceValues(ctx, choiceResp.Choice.Name)
	if err != nil || len(items) != 2 {
		t.Fatalf("resolve choice: err=%v len=%d", err, len(items))
	}
	_, err = svc.UpdateChoice(ctx, &apiv1schema.UpdateChoiceRequest{
		Id:            choiceName,
		ReplaceValues: true,
		Values: []*apiv1schema.ChoiceItem{
			{Value: "active"},
			{Value: "inactive"},
			{Value: "pending"},
		},
	})
	if err != nil {
		t.Fatalf("replace choice add value: %v", err)
	}
	items, err = svc.ResolveChoiceValues(ctx, choiceName)
	if err != nil || len(items) != 3 {
		t.Fatalf("choice after replace add: err=%v len=%d", err, len(items))
	}
	_, err = svc.UpdateChoice(ctx, &apiv1schema.UpdateChoiceRequest{
		Id:            choiceName,
		ReplaceValues: true,
		Values: []*apiv1schema.ChoiceItem{
			{Value: "active"},
			{Value: "inactive"},
		},
	})
	if err != nil {
		t.Fatalf("replace choice remove value: %v", err)
	}
	items, err = svc.ResolveChoiceValues(ctx, choiceName)
	if err != nil || len(items) != 2 {
		t.Fatalf("choice after replace remove: err=%v len=%d", err, len(items))
	}
	_, err = svc.UpdateChoice(ctx, &apiv1schema.UpdateChoiceRequest{
		Id:     choiceName,
		Values: []*apiv1schema.ChoiceItem{{Value: "archived"}},
	})
	if err != nil {
		t.Fatalf("append choice value: %v", err)
	}
	items, err = svc.ResolveChoiceValues(ctx, choiceName)
	if err != nil || len(items) != 3 {
		t.Fatalf("choice after append: err=%v len=%d", err, len(items))
	}

	// --- expand relationship ---
	listResp, err := svc.ListRows(ctx, &row.ListRowsRequest{
		TableId: vendorTable, PageSize: 10,
		ExpandColumnIds: []string{relCol.Column.Id},
	})
	if err != nil {
		t.Fatalf("list with expand: %v", err)
	}
	if len(listResp.Rows) == 0 || listResp.Rows[0].Cells[relCol.Column.Id] == nil {
		t.Fatal("relationship expand missing")
	}

	// --- rename table ---
	renamed := vendorTable + "_renamed"
	_, err = svc.RenameTable(ctx, &apiv1schema.RenameTableRequest{Id: vendorTable, NewName: renamed})
	if err != nil {
		t.Fatalf("rename table: %v", err)
	}

	// cleanup renamed table
	_, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: renamed})
	_, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: orderTable})
}

func TestIntegrationTypesList(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	types, err := svc.ListTypes(ctx, &platform.ListTypesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"int8": false, "text": false, "formula": false, "rollup": false, "relation_fk": false}
	for _, ty := range types.Types {
		if _, ok := want[ty.Name]; ok {
			want[ty.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("type %q not seeded", name)
		}
	}
}

func TestIntegrationTableRowIDType(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	tableName := testutil.UniqueName("int8_ids")
	resp, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: tableName, IdType: "int8"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Table.IdType != "int8" {
		t.Fatalf("idType=%q", resp.Table.IdType)
	}
	defer func() { _, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: tableName}) }()

	col, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: tableName, Name: "title", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: tableName,
		Cells:   map[string]*apiv1.Value{col.Column.Id: apiv1.StringValue("hello")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Row.Id == "" {
		t.Fatal("expected numeric row id")
	}
	schemaResp, err := svc.GetTableSchema(ctx, &apiv1schema.GetTableSchemaRequest{TableId: tableName})
	if err != nil || schemaResp.Table.IdType != "int8" {
		t.Fatalf("schema idType=%q err=%v", schemaResp.Table.IdType, err)
	}
}

func TestIntegrationColumnTypes(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	tableName := testutil.UniqueName("types_demo")
	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: tableName}); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = svc.DeleteTable(ctx, &apiv1schema.DeleteTableRequest{Id: tableName}) }()

	typeSpecs := []struct {
		name   string
		typeID string
	}{
		{"c_int8", "int8"},
		{"c_double", "double"},
		{"c_precision", "precision"},
		{"c_text", "text"},
		{"c_ts", "timestamptz"},
		{"c_bool", "bool"},
		{"c_jsonb", "jsonb"},
		{"c_uuid", "uuid"},
		{"c_int8_arr", "int8_array"},
		{"c_text_arr", "text_array"},
	}
	for i, spec := range typeSpecs {
		if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
			TableId: tableName, Name: spec.name, TypeId: spec.typeID, Position: int32(i + 1),
		}); err != nil {
			t.Fatalf("add column %s (%s): %v", spec.name, spec.typeID, err)
		}
	}
	cols, err := svc.ListColumns(ctx, &apiv1schema.ListColumnsRequest{TableId: tableName})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols.Columns) < len(typeSpecs) {
		t.Fatalf("expected %d columns, got %d", len(typeSpecs), len(cols.Columns))
	}
}

func TestIntegrationSaveGraph(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	orderTable := testutil.UniqueName("order")
	itemTable := testutil.UniqueName("order_item")

	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: orderTable}); err != nil {
		t.Fatalf("create order table: %v", err)
	}
	if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: itemTable}); err != nil {
		t.Fatalf("create item table: %v", err)
	}

	amountCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "amount", TypeId: "precision", Position: 1,
	})
	if err != nil {
		t.Fatalf("add amount: %v", err)
	}

	qtyCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "qty", TypeId: "int8", Position: 1,
	})
	if err != nil {
		t.Fatalf("add qty: %v", err)
	}
	goodsCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods_id", TypeId: "text", Position: 2,
	})
	if err != nil {
		t.Fatalf("add goods_id: %v", err)
	}
	orderLinkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "order_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatalf("add order_id: %v", err)
	}

	_, err = svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "items", TypeId: "relationship", Position: 2,
		Config: map[string]any{
			"target_table_id": itemTable,
			"link_column_id":  orderLinkCol.Column.Id,
			"cardinality":     "many",
		},
	})
	if err != nil {
		t.Fatalf("add items relationship: %v", err)
	}

	// create with nested items
	createBody := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(`{
		"amount": 100,
		"items": [
			{ "qty": 2, "goods_id": "g1" },
			{ "qty": 1, "goods_id": "g2" }
		]
	}`), createBody); err != nil {
		t.Fatal(err)
	}
	createBody.TableId = orderTable

	created, err := svc.SaveGraph(ctx, createBody)
	if err != nil {
		t.Fatalf("saveGraph create: %v", err)
	}
	if created["id"] == "" || created["id"] == nil {
		t.Fatal("missing root id")
	}
	items := saveGraphItems(t, created)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	item1ID, _ := items[0]["id"].(string)

	// update: change amount, update one item, add one, deleteMissing removes others
	updateRaw := fmt.Sprintf(`{
		"id": %q,
		"amount": 200,
		"_sync": { "items": "replace" },
		"items": [
			{ "id": %q, "qty": 5, "goods_id": "g1-upd" },
			{ "qty": 3, "goods_id": "g3" }
		]
	}`, created["id"], item1ID)
	updateBody := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(updateRaw), updateBody); err != nil {
		t.Fatal(err)
	}
	updateBody.TableId = orderTable
	updated, err := svc.SaveGraph(ctx, updateBody)
	if err != nil {
		t.Fatalf("saveGraph update: %v", err)
	}
	if updated["amount"] != float64(200) && updated["amount"] != 200 {
		t.Fatalf("amount not updated: %+v", updated)
	}
	if len(saveGraphItems(t, updated)) != 2 {
		t.Fatalf("expected 2 items after update, got %+v", updated["items"])
	}

	list, err := svc.ListRows(ctx, &row.ListRowsRequest{
		TableId:         itemTable,
		ExpandColumnIds: []string{},
		PageSize:        50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) != 2 {
		t.Fatalf("expected 2 item rows in DB, got %d", len(list.Rows))
	}
	_ = amountCol
	_ = orderLinkCol
	_ = qtyCol
	_ = goodsCol
}

func TestIntegrationSaveGraphLookupWrite(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	goodsTable := testutil.UniqueName("goods")
	orderTable := testutil.UniqueName("order")
	itemTable := testutil.UniqueName("order_item")

	for _, name := range []string{goodsTable, orderTable, itemTable} {
		if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: name}); err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
	}

	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: goodsTable, Name: "name", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	goodsApple, err := svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: goodsTable,
		Cells:   map[string]*apiv1.Value{"name": apiv1.StringValue("Apple")},
	})
	if err != nil {
		t.Fatalf("create goods Apple: %v", err)
	}
	if _, err := svc.CreateRow(ctx, &row.CreateRowRequest{
		TableId: goodsTable,
		Cells:   map[string]*apiv1.Value{"name": apiv1.StringValue("Banana")},
	}); err != nil {
		t.Fatalf("create goods Banana: %v", err)
	}

	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "amount", TypeId: "precision", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "qty", TypeId: "int8", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	goodsFKCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods_id", TypeId: "uuid", Position: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	orderLinkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "order_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	goodsRelCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods", TypeId: "relationship", Position: 4,
		Config: map[string]any{
			"target_table_id":  goodsTable,
			"target_column_id": goodsFKCol.Column.Id,
			"cardinality":      "one",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods_name", TypeId: "lookup", Position: 5,
		Config: map[string]any{
			"relation_column_id": goodsRelCol.Column.Id,
			"target_column_id":   "name",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "items", TypeId: "relationship", Position: 2,
		Config: map[string]any{
			"target_table_id": itemTable,
			"link_column_id":  orderLinkCol.Column.Id,
			"cardinality":     "many",
		},
	}); err != nil {
		t.Fatal(err)
	}

	createBody := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(`{
		"amount": 100,
		"items": [
			{ "qty": 2, "goods_name": "Apple" },
			{ "qty": 1, "goods_name": "Banana" }
		]
	}`), createBody); err != nil {
		t.Fatal(err)
	}
	createBody.TableId = orderTable
	created, err := svc.SaveGraph(ctx, createBody)
	if err != nil {
		t.Fatalf("saveGraph create: %v", err)
	}
	if len(saveGraphItems(t, created)) != 2 {
		t.Fatalf("expected 2 items, got %+v", created["items"])
	}
	item1 := saveGraphItems(t, created)[0]
	item1ID, _ := item1["id"].(string)
	if item1["goods_id"] != goodsApple.Row.Id {
		t.Fatalf("expected goods_id=%q, got %+v", goodsApple.Row.Id, item1["goods_id"])
	}

	updateRaw := fmt.Sprintf(`{
		"id": %q,
		"amount": 200,
		"_sync": { "items": "replace" },
		"items": [
			{ "id": %q, "qty": 5, "goods_name": "Banana" },
			{ "qty": 3, "goods_name": "Apple" }
		]
	}`, created["id"], item1ID)
	updateBody := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(updateRaw), updateBody); err != nil {
		t.Fatal(err)
	}
	updateBody.TableId = orderTable
	updated, err := svc.SaveGraph(ctx, updateBody)
	if err != nil {
		t.Fatalf("saveGraph update: %v", err)
	}
	if updated["amount"] != float64(200) && updated["amount"] != 200 {
		t.Fatalf("amount not updated: %+v", updated)
	}
}

func TestIntegrationSaveGraphOneRelCreate(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	warehouseTable := testutil.UniqueName("warehouse")
	orderTable := testutil.UniqueName("order")

	for _, name := range []string{warehouseTable, orderTable} {
		if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: name}); err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: warehouseTable, Name: "name", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "amount", TypeId: "precision", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	warehouseFKCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "warehouse_id", TypeId: "uuid", Position: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	warehouseRelCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "warehouse", TypeId: "relationship", Position: 3,
		Config: map[string]any{
			"target_table_id":  warehouseTable,
			"target_column_id": warehouseFKCol.Column.Id,
			"cardinality":      "one",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = warehouseRelCol

	body := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(`{
		"amount": 100,
		"warehouse": { "name": "华东仓" }
	}`), body); err != nil {
		t.Fatal(err)
	}
	body.TableId = orderTable
	created, err := svc.SaveGraph(ctx, body)
	if err != nil {
		t.Fatalf("saveGraph: %v", err)
	}
	warehouse := saveGraphNested(t, created, "warehouse")
	warehouseID, _ := warehouse["id"].(string)
	if warehouseID == "" {
		t.Fatal("missing created warehouse id")
	}
	if created["warehouse_id"] != warehouseID {
		t.Fatalf("warehouse_id=%v want %q", created["warehouse_id"], warehouseID)
	}
}

func TestIntegrationSaveGraphNestedOneRelCreate(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	goodsTable := testutil.UniqueName("goods")
	orderTable := testutil.UniqueName("order")
	itemTable := testutil.UniqueName("order_item")

	for _, name := range []string{goodsTable, orderTable, itemTable} {
		if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: name}); err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: goodsTable, Name: "name", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "amount", TypeId: "precision", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "qty", TypeId: "int8", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	goodsFKCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods_id", TypeId: "uuid", Position: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	orderLinkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "order_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods", TypeId: "relationship", Position: 4,
		Config: map[string]any{
			"target_table_id":  goodsTable,
			"target_column_id": goodsFKCol.Column.Id,
			"cardinality":      "one",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "items", TypeId: "relationship", Position: 2,
		Config: map[string]any{
			"target_table_id": itemTable,
			"link_column_id":  orderLinkCol.Column.Id,
			"cardinality":     "many",
		},
	}); err != nil {
		t.Fatal(err)
	}

	body := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(`{
		"amount": 100,
		"items": [
			{ "qty": 2, "goods": { "name": "Apple" } }
		]
	}`), body); err != nil {
		t.Fatal(err)
	}
	body.TableId = orderTable
	created, err := svc.SaveGraph(ctx, body)
	if err != nil {
		t.Fatalf("saveGraph: %v", err)
	}
	item := saveGraphItems(t, created)[0]
	goodsID, _ := item["goods_id"].(string)
	if goodsID == "" {
		t.Fatalf("missing goods_id on item: %+v", item)
	}
	list, err := svc.ListRows(ctx, &row.ListRowsRequest{TableId: goodsTable, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) != 1 || list.Rows[0].Id != goodsID {
		t.Fatalf("expected one goods row id=%q, got %+v", goodsID, list.Rows)
	}
}

func TestIntegrationSaveGraphCreateOrderShape(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	supplyTable := testutil.UniqueName("supply")
	goodsTable := testutil.UniqueName("goods")
	orderTable := testutil.UniqueName("order")
	itemTable := testutil.UniqueName("order_item")

	for _, name := range []string{supplyTable, goodsTable, orderTable, itemTable} {
		if _, err := svc.CreateTable(ctx, &apiv1schema.CreateTableRequest{Name: name}); err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: supplyTable, Name: "code", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: goodsTable, Name: "name", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "order_remark", TypeId: "text", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	supplyFKCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "supply_id", TypeId: "uuid", Position: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	supplyRelCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "supply", TypeId: "relationship", Position: 3,
		Config: map[string]any{
			"target_table_id":  supplyTable,
			"target_column_id": supplyFKCol.Column.Id,
			"cardinality":      "one",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "supply_code", TypeId: "lookup", Position: 4,
		Config: map[string]any{
			"relation_column_id": supplyRelCol.Column.Id,
			"target_column_id":   "code",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "qty", TypeId: "int8", Position: 1,
	}); err != nil {
		t.Fatal(err)
	}
	goodsFKCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods_id", TypeId: "uuid", Position: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	orderLinkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "order_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: itemTable, Name: "goods", TypeId: "relationship", Position: 4,
		Config: map[string]any{
			"target_table_id":  goodsTable,
			"target_column_id": goodsFKCol.Column.Id,
			"cardinality":      "one",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "items", TypeId: "relationship", Position: 5,
		Config: map[string]any{
			"target_table_id": itemTable,
			"link_column_id":  orderLinkCol.Column.Id,
			"cardinality":     "many",
		},
	}); err != nil {
		t.Fatal(err)
	}

	body := &graph.SaveGraphRequest{}
	if err := json.Unmarshal([]byte(`{
		"order_remark": "rush",
		"supply": { "code": "SUP-001" },
		"items": [
			{ "qty": 2, "goods": { "name": "Apple" } }
		]
	}`), body); err != nil {
		t.Fatal(err)
	}
	body.TableId = orderTable
	created, err := svc.SaveGraph(ctx, body)
	if err != nil {
		t.Fatalf("saveGraph: %v", err)
	}
	if created["order_remark"] != "rush" {
		t.Fatalf("order_remark=%+v", created["order_remark"])
	}
	supply := saveGraphNested(t, created, "supply")
	supplyID, _ := supply["id"].(string)
	if supplyID == "" {
		t.Fatalf("missing supply row: %+v", created)
	}
	if created["supply_id"] != supplyID {
		t.Fatalf("supply_id=%v want %q", created["supply_id"], supplyID)
	}
	if len(saveGraphItems(t, created)) != 1 {
		t.Fatalf("items=%+v", created["items"])
	}
}
