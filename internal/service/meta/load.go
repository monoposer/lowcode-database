package meta

import (
	"context"
	"fmt"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (r *Read) LoadColumns(ctx context.Context, tableID string) ([]shared.ColumnMeta, string, string, error) {
	return catalog.New(r.B).LoadColumns(ctx, tableID)
}

func (r *Read) LoadAllColumnMeta(ctx context.Context, tableID string) ([]shared.FullColumnMeta, string, string, error) {
	return catalog.New(r.B).LoadAllColumnMeta(ctx, tableID)
}

func (r *Read) ResolveColumnName(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	return schema.New(r.B).ResolveColumnName(ctx, tenantID, tableKey, ref)
}

func (r *Read) NormalizeColumnNames(ctx context.Context, tenantID, tableKey string, refs []string) ([]string, error) {
	return schema.New(r.B).NormalizeColumnNames(ctx, tenantID, tableKey, refs)
}

func (r *Read) ColumnPgColumnByRef(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	return schema.New(r.B).ColumnPgColumnByRef(ctx, tenantID, tableKey, ref)
}

func (r *Read) LoadTableIDPgType(ctx context.Context, schemaName, tableName string) (string, error) {
	return schema.New(r.B).LoadTableIDPgType(ctx, schemaName, tableName)
}

func (r *Read) ResolveChoiceColumnRef(ctx context.Context, tid, typeID string, cfg map[string]any) (logicalName string, ok bool, err error) {
	return catalog.New(r.B).ResolveChoiceColumnRef(ctx, tid, typeID, cfg)
}

func (r *Read) ChoiceColumnDDLType(ctx context.Context, tid, choiceRef string) (string, error) {
	return catalog.New(r.B).ChoiceColumnDDLType(ctx, tid, choiceRef)
}

func (r *Read) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	return catalog.New(r.B).ColumnPgTypeSQL(ctx, tid, typeID, cfg)
}

func (r *Read) ResolveDataSourceRef(ctx context.Context, tableRef, dsRef string) (tableID, dsName string, err error) {
	if dsRef == "" {
		return "", "", fmt.Errorf("data source name is required")
	}
	if tableRef == "" {
		return "", "", fmt.Errorf("table_id is required")
	}
	tableID, err = r.B.ResolveTableName(ctx, tableRef)
	if err != nil {
		return "", "", err
	}
	if err := shared.ValidateTableName(dsRef); err != nil {
		return "", "", fmt.Errorf("data source name: %w", err)
	}
	return tableID, dsRef, nil
}

func (r *Read) LoadRelationshipColumns(ctx context.Context, tableID string, columnIDs []string) ([]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadRelationshipColumns(ctx, tableID, columnIDs)
}

func (r *Read) LoadManyRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadManyRelationshipColumns(ctx, tableID)
}

func (r *Read) LoadOneRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadOneRelationshipColumns(ctx, tableID)
}

func (r *Read) LoadLookupWriteSpecs(ctx context.Context, tableID string) (map[string]shared.LookupWriteSpec, error) {
	return schema.New(r.B).LoadLookupWriteSpecs(ctx, tableID)
}

func (r *Read) ListTableIndexes(ctx context.Context, tableID, schemaName, tableName string) ([]*apiv1schema.Index, error) {
	rows, err := catalog.New(r.B).ListPGIndexes(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	return catalog.New(r.B).PGIndexesToAPI(ctx, tableID, schemaName, tableName, rows)
}
