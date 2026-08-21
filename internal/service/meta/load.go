package meta

import (
	"context"
	"fmt"
	"github.com/monoposer/lowcode-database/internal/numeric"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (r *Read) LoadColumns(ctx context.Context, tableName string) ([]shared.ColumnMeta, string, string, error) {
	return catalog.New(r.B).LoadColumns(ctx, tableName)
}

func (r *Read) LoadAllColumnMeta(ctx context.Context, tableName string) ([]shared.FullColumnMeta, string, string, error) {
	return catalog.New(r.B).LoadAllColumnMeta(ctx, tableName)
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

func (r *Read) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	return catalog.New(r.B).ColumnPgTypeSQL(ctx, tid, typeID, cfg)
}

func (r *Read) NumericSpecForType(ctx context.Context, typeID string) (numeric.Spec, bool) {
	tid, err := r.B.TenantID(ctx)
	if err != nil {
		return numeric.Spec{}, false
	}
	return catalog.New(r.B).NumericSpecForType(ctx, tid, typeID)
}

func (r *Read) ResolveQueryRef(ctx context.Context, tableRef, dsRef string) (tableName, dsName string, err error) {
	if dsRef == "" {
		return "", "", fmt.Errorf("query name is required")
	}
	if tableRef == "" {
		return "", "", fmt.Errorf("table_name is required")
	}
	tableName, err = r.B.ResolveTableName(ctx, tableRef)
	if err != nil {
		return "", "", err
	}
	if err := shared.ValidateTableName(dsRef); err != nil {
		return "", "", fmt.Errorf("query name: %w", err)
	}
	return tableName, dsRef, nil
}

func (r *Read) LoadRelationshipColumns(ctx context.Context, tableName string, columnIDs []string) ([]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadRelationshipColumns(ctx, tableName, columnIDs)
}

func (r *Read) LoadManyRelationshipColumns(ctx context.Context, tableName string) (map[string]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadManyRelationshipColumns(ctx, tableName)
}

func (r *Read) LoadOneRelationshipColumns(ctx context.Context, tableName string) (map[string]shared.RelationshipColumn, error) {
	return schema.New(r.B).LoadOneRelationshipColumns(ctx, tableName)
}

func (r *Read) LoadLookupWriteSpecs(ctx context.Context, tableName string) (map[string]shared.LookupWriteSpec, error) {
	return schema.New(r.B).LoadLookupWriteSpecs(ctx, tableName)
}

func (r *Read) ListTableIndexes(ctx context.Context, tableName, schemaName, physicalName string) ([]*catalog.Index, error) {
	rows, err := catalog.New(r.B).ListPGIndexes(ctx, schemaName, physicalName)
	if err != nil {
		return nil, err
	}
	return catalog.New(r.B).PGIndexesToAPI(ctx, tableName, schemaName, physicalName, rows)
}
