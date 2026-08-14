package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/platform"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

func TestIntegrationFullWorkflow(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	// --- tables ---
	vendorTable := testutil.UniqueName("vendor")
	orderTable := testutil.UniqueName("order")

	if _, err := svc.CreateTable(ctx, &schema.Table{Name: vendorTable}); err != nil {
		t.Fatalf("create vendor table: %v", err)
	}
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: orderTable}); err != nil {
		t.Fatalf("create order table: %v", err)
	}

	// --- columns: scalar types ---
	nameCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "name", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatalf("add text column: %v", err)
	}
	scoreCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "score", TypeId: "number", Position: 2,
	})
	if err != nil {
		t.Fatalf("add double column: %v", err)
	}
	activeCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "active", TypeId: "boolean", Position: 3,
	})
	if err != nil {
		t.Fatalf("add bool column: %v", err)
	}
	metaCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "meta", TypeId: "jsonb", Position: 4,
	})
	if err != nil {
		t.Fatalf("add jsonb column: %v", err)
	}
	_ = nameCol
	_ = scoreCol
	_ = activeCol
	_ = metaCol

	// int8 id column on vendor for FK demo
	vendorIDCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "vendor_num", TypeId: "number", Position: 5,
	})
	if err != nil {
		t.Fatalf("add int8 column: %v", err)
	}

	amountCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orderTable, Name: "amount", TypeId: "number", Position: 1,
	})
	if err != nil {
		t.Fatalf("add precision column: %v", err)
	}

	// physical id on order pointing at vendor; link column is virtual
	fkCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orderTable, Name: "vendor_ref", TypeId: "number", Position: 2,
	})
	if err != nil {
		t.Fatalf("add vendor_ref: %v", err)
	}

	// relationship many: orders linked by vendor uuid id
	linkCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: orderTable, Name: "vendor_id", TypeId: "text", Position: 3,
	})
	if err != nil {
		t.Fatalf("add uuid column: %v", err)
	}

	_, err = svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "orders", TypeId: "link", Position: 6,
		Config: map[string]any{
			"target_table_name": orderTable,
			"link_column_id":  linkCol.Id,
			"cardinality":     "many",
		},
	})
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	// formula on vendor
	formulaCol, err := svc.AddColumn(ctx, &schema.Column{
		TableName: vendorTable, Name: "double_score", TypeId: "formula", Position: 7,
		Config: map[string]any{"expression": "{{score}} * 2"},
	})
	if err != nil {
		t.Fatalf("add formula: %v", err)
	}

	// --- relation registry ---
	_, err = svc.CreateRelation(ctx, &schema.Relation{
		Name:           testutil.UniqueName("order_vendor"),
		Kind:           "MANY_TO_ONE",
		SourceTableName:  orderTable,
		SourceColumnId: fkCol.Name,
		TargetTableName:  vendorTable,
		TargetColumnId: vendorIDCol.Name,
	})
	if err != nil {
		t.Fatalf("create relation: %v", err)
	}

	// --- rows ---
	vendorRow, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: vendorTable,
		Cells: map[string]*shared.Value{
			nameCol.Id:     shared.StringValue("Acme"),
			scoreCol.Id:    shared.NumberValue(10),
			activeCol.Id:   shared.BoolValue(true),
			metaCol.Id:     shared.JsonValue(map[string]any{"tier": "gold"}),
			vendorIDCol.Id: shared.NumberValue(1001),
		},
	})
	if err != nil {
		t.Fatalf("create vendor row: %v", err)
	}

	_, err = svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: orderTable,
		Cells: map[string]*shared.Value{
			amountCol.Id: shared.NumberValue(99.5),
			fkCol.Id:     shared.NumberValue(1001),
			linkCol.Id:   shared.StringValue(vendorRow.Row.Id),
		},
	})
	if err != nil {
		t.Fatalf("create order row: %v", err)
	}

	// --- index ---
	_, err = svc.CreateIndex(ctx, &catalog.Index{
		TableName: vendorTable, Name: "score_idx", ColumnIds: []string{scoreCol.Id},
	})
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	idxList, err := svc.ListIndexes(ctx, vendorTable)
	if err != nil || len(idxList) == 0 {
		t.Fatalf("list indexes: %v len=%d", err, len(idxList))
	}

	// --- saved query ---
	dsResp, err := svc.CreateQuery(ctx, &platform.Query{
		Name:      testutil.UniqueName("active_vendors"),
		Label:     "Active Vendors",
		TableName:   vendorTable,
		ColumnIds: []string{nameCol.Name, scoreCol.Name, formulaCol.Name},
		Filter: map[string]any{
			"type": "EQ", "attr": activeCol.Name, "val": true,
		},
		Sort: []*shared.SortOrder{{Attribute: scoreCol.Name, SortOrder: "DESC"}},
	})
	if err != nil {
		t.Fatalf("create query: %v", err)
	}

	dsQuery, err := svc.ExecuteQuery(ctx, &platform.ExecuteQueryRequest{
		TableName:  vendorTable,
		QueryId:  dsResp.Name,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if len(dsQuery.Rows) == 0 {
		t.Fatal("query returned no rows")
	}
	if dsQuery.Rows[0].Cells[formulaCol.Id] == nil {
		t.Fatal("formula column missing in query")
	}

	// --- query rows with filter ---
	qrows, err := svc.QueryRows(ctx, &data.QueryRowsRequest{
		TableName:  vendorTable,
		Filter:   map[string]any{"type": "EQ", "attr": nameCol.Id, "val": "Acme"},
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	if len(qrows.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(qrows.Rows))
	}

	// --- ER diagram ---
	er, err := svc.GetERDiagram(ctx)
	if err != nil {
		t.Fatalf("er diagram: %v", err)
	}
	if len(er.Nodes) < 2 {
		t.Fatalf("expected >=2 nodes, got %d", len(er.Nodes))
	}
	if len(er.Edges) == 0 {
		t.Fatal("expected edges in ER diagram")
	}

	// --- list tables / schema ---
	tables, err := svc.ListTables(ctx)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	found := false
	for _, tbl := range tables {
		if tbl.Id == vendorTable {
			found = true
		}
	}
	if !found {
		t.Fatal("vendor table not in list")
	}

	_, cols, _, err := svc.GetTableSchema(ctx, vendorTable)
	if err != nil {
		t.Fatalf("get schema: %v", err)
	}
	if len(cols) < 5 {
		t.Fatalf("expected many columns, got %d", len(cols))
	}

	// --- rename table ---
	renamed := vendorTable + "_renamed"
	_, err = svc.RenameTable(ctx, vendorTable, renamed)
	if err != nil {
		t.Fatalf("rename table: %v", err)
	}

	// cleanup renamed table
	_ = svc.DeleteTable(ctx, renamed)
	_ = svc.DeleteTable(ctx, orderTable)
}

func TestIntegrationTypesList(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	types, err := svc.ListTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"number": false, "text": false, "datetime": false, "formula": false, "rollup": false, "link": false}
	for _, ty := range types {
		if _, ok := want[ty.Id]; ok {
			want[ty.Id] = true
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
	resp, err := svc.CreateTable(ctx, &schema.Table{Name: tableName, IdType: "number"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IdType != "number" {
		t.Fatalf("idType=%q", resp.IdType)
	}
	defer func() { _ = svc.DeleteTable(ctx, tableName) }()

	col, err := svc.AddColumn(ctx, &schema.Column{
		TableName: tableName, Name: "title", TypeId: "text", Position: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := svc.CreateRow(ctx, &data.CreateRowRequest{
		TableName: tableName,
		Cells:   map[string]*shared.Value{col.Id: shared.StringValue("hello")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Row.Id == "" {
		t.Fatal("expected numeric row id")
	}
	tbl, _, _, err := svc.GetTableSchema(ctx, tableName)
	if err != nil || tbl.IdType != "number" {
		t.Fatalf("schema idType=%q err=%v", tbl.IdType, err)
	}
}

func TestIntegrationColumnTypes(t *testing.T) {
	svc, cleanup := testutil.SetupIntegration(t)
	defer cleanup()
	ctx := testutil.Ctx()

	tableName := testutil.UniqueName("types_demo")
	if _, err := svc.CreateTable(ctx, &schema.Table{Name: tableName}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.DeleteTable(ctx, tableName) }()

	selectName := testutil.UniqueName("select")
	multiName := testutil.UniqueName("multi_select")
	if _, err := svc.CreateColumnType(ctx, &catalog.ColumnTypeDef{
		Name:  selectName,
		Label: "Select",
		Spec:  &columntype.ColumnTypeSpec{PgType: "text"},
	}); err != nil {
		t.Fatalf("create select columnType: %v", err)
	}
	if _, err := svc.CreateColumnType(ctx, &catalog.ColumnTypeDef{
		Name:  multiName,
		Label: "Multi Select",
		Spec:  &columntype.ColumnTypeSpec{PgType: "text", Array: true},
	}); err != nil {
		t.Fatalf("create multi_select columnType: %v", err)
	}

	typeSpecs := []struct {
		name   string
		typeID string
	}{
		{"c_number", "number"},
		{"c_text", "text"},
		{"c_datetime", "datetime"},
		{"c_bool", "boolean"},
		{"c_jsonb", "jsonb"},
		{"c_select", selectName},
		{"c_multi", multiName},
	}
	for i, spec := range typeSpecs {
		if _, err := svc.AddColumn(ctx, &schema.Column{
			TableName: tableName, Name: spec.name, TypeId: spec.typeID, Position: int32(i + 1),
		}); err != nil {
			t.Fatalf("add column %s (%s): %v", spec.name, spec.typeID, err)
		}
	}
	cols, err := svc.ListColumns(ctx, tableName)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) < len(typeSpecs) {
		t.Fatalf("expected %d columns, got %d", len(typeSpecs), len(cols))
	}

	if pg := svc.ColumnPgTypeSQL(ctx, "test", multiName, nil); pg != "text[]" {
		t.Fatalf("multi_select pgType=%q want text[]", pg)
	}
	if pg := svc.ColumnPgTypeSQL(ctx, "test", selectName, nil); pg != "text" {
		t.Fatalf("select pgType=%q want text", pg)
	}

	types, err := svc.ListTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var multiAPI *catalog.Type
	for _, ty := range types {
		if ty.Id == multiName {
			multiAPI = ty
			break
		}
	}
	if multiAPI == nil {
		t.Fatal("multi_select missing from ListTypes")
	}
	if multiAPI.PgType != "text[]" {
		t.Fatalf("ListTypes pgType=%q want text[]", multiAPI.PgType)
	}
	if multiAPI.Config["array"] != true {
		t.Fatalf("ListTypes config.array=%v want true", multiAPI.Config["array"])
	}
}
