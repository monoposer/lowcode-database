package service_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/platform"
	"github.com/monoposer/lowcode-database/internal/apiv1/query"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/testutil"
)

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

	// physical id on order pointing at vendor; link column is virtual
	fkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "vendor_ref", TypeId: "bigint", Position: 2,
	})
	if err != nil {
		t.Fatalf("add vendor_ref: %v", err)
	}

	// relationship many: orders linked by vendor uuid id
	linkCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: orderTable, Name: "vendor_id", TypeId: "uuid", Position: 3,
	})
	if err != nil {
		t.Fatalf("add uuid column: %v", err)
	}

	_, err = svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "orders", TypeId: "link", Position: 6,
		Config: map[string]any{
			"target_table_id": orderTable,
			"link_column_id":  linkCol.Column.Id,
			"cardinality":     "many",
		},
	})
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	// formula on vendor
	formulaCol, err := svc.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: vendorTable, Name: "double_score", TypeId: "formula", Position: 7,
		Config: map[string]any{"expression": "{{score}} * 2"},
	})
	if err != nil {
		t.Fatalf("add formula: %v", err)
	}

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
		TableId: vendorTable, Name: "score_idx", ColumnIds: []string{scoreCol.Column.Id},
	})
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	idxList, err := svc.ListIndexes(ctx, &apiv1schema.ListIndexesRequest{TableId: vendorTable})
	if err != nil || len(idxList.Indexes) == 0 {
		t.Fatalf("list indexes: %v len=%d", err, len(idxList.Indexes))
	}

	// --- saved query ---
	dsResp, err := svc.CreateQuery(ctx, &query.CreateQueryRequest{
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
		t.Fatalf("create query: %v", err)
	}

	dsQuery, err := svc.ExecuteQuery(ctx, &query.ExecuteQueryRequest{
		TableId:  vendorTable,
		QueryId:  dsResp.Query.Name,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if len(dsQuery.Rows) == 0 {
		t.Fatal("query returned no rows")
	}
	if dsQuery.Rows[0].Cells[formulaCol.Column.Id] == nil {
		t.Fatal("formula column missing in query")
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
	want := map[string]bool{"number": false, "text": false, "datetime": false, "formula": false, "rollup": false, "link": false}
	for _, ty := range types.Types {
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
