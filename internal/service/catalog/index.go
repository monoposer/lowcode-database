package catalog

import (
	"context"
	"fmt"
	"strings"

)

func (s *Catalog) ListIndexes(ctx context.Context, tableName string) ([]*Index, error) {
	if tableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	tableName, err := s.B.ResolveTableName(ctx, tableName)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	metaRows, err := s.listIndexMeta(ctx, tid, baseID, tableName)
	if err != nil {
		return nil, err
	}
	var indexes []*Index
	for _, r := range metaRows {
		indexes = append(indexes, s.indexMetaToAPI(r))
	}
	return indexes, nil
}

func (s *Catalog) GetIndex(ctx context.Context, tableName, id string) (*Index, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta, err := s.findIndexMeta(ctx, tid, baseID, tableName, id)
	if err != nil {
		return nil, fmt.Errorf("index not found")
	}
	return s.indexMetaToAPI(*meta), nil
}

func (s *Catalog) CreateIndex(ctx context.Context, req *Index) (*Index, error) {
	if req.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	cols, _, tableName, err := s.LoadColumns(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	resolvedTable, err := s.B.ResolveTableName(ctx, req.TableName)
	if err != nil {
		return nil, err
	}

	colIDSet := make(map[string]struct{}, len(req.ColumnIds))
	for _, id := range req.ColumnIds {
		colIDSet[id] = struct{}{}
	}
	var columnIDs []string
	var colNames []string
	for _, c := range cols {
		if _, ok := colIDSet[c.Id]; ok {
			columnIDs = append(columnIDs, c.Id)
			colNames = append(colNames, c.Name)
		}
	}
	if len(columnIDs) == 0 {
		return nil, fmt.Errorf("no valid columns for index")
	}

	logicalName, err := sanitizePgIdent(req.Name)
	if err != nil {
		return nil, err
	}
	pgIndex, err := indexSQLName(tableName, logicalName)
	if err != nil {
		return nil, err
	}

	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	vtID, err := s.B.Tenants.TableVTID(ctx, tenantID, baseID, resolvedTable)
	if err != nil {
		return nil, err
	}
	expr := fmt.Sprintf(`(data->>'%s')`, strings.ReplaceAll(colNames[0], "'", "''"))
	if len(colNames) > 1 {
		parts := make([]string, len(colNames))
		for i, n := range colNames {
			parts[i] = fmt.Sprintf(`(data->>'%s')`, strings.ReplaceAll(n, "'", "''"))
		}
		expr = "(" + strings.Join(parts, ", ") + ")"
	}
	if err := s.insertVRIndexMeta(ctx, tid, baseID, resolvedTable, logicalName, pgIndex, vtID, expr, "btree", columnIDs, req.IsUnique); err != nil {
		return nil, fmt.Errorf("insert lc_indexes: %w", err)
	}
	for _, n := range colNames {
		_, _ = s.B.Tenants.MetaPool().Exec(ctx, `
			UPDATE lc_columns SET config = COALESCE(config,'{}'::jsonb) || '{"need_index":true}'::jsonb
			WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`, tid, baseID, resolvedTable, n)
	}
	meta, err := s.getIndexMeta(ctx, tid, baseID, resolvedTable, logicalName)
	if err != nil {
		return &Index{
			Id: logicalName, TableName: resolvedTable, Name: logicalName, PgIndex: pgIndex,
			ColumnIds: columnIDs, IsUnique: req.IsUnique,
		}, nil
	}
	return s.indexMetaToAPI(*meta), nil
}

func (s *Catalog) DeleteIndex(ctx context.Context, tableName, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	meta, err := s.findIndexMeta(ctx, tid, baseID, tableName, id)
	if err != nil {
		return fmt.Errorf("index not found")
	}
	_, _ = s.B.Tenants.MetaPool().Exec(ctx, `
		UPDATE lc_indexes SET migrate_status = 'drop_pending', updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
		tid, baseID, meta.TableName, meta.Name)
	return nil
}
