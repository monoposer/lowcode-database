package catalog

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
	"strings"
)

func (s *Catalog) ListIndexes(ctx context.Context, req *apiv1schema.ListIndexesRequest) (*apiv1schema.ListIndexesResponse, error) {
	tableID, schemaName, tableName, err := s.B.LoadTablePhysical(ctx, req.TableId)
	if err != nil {
		return nil, err
	}
	pgRows, err := s.ListPGIndexes(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	indexes, err := s.PGIndexesToAPI(ctx, tableID, schemaName, tableName, pgRows)
	if err != nil {
		return nil, err
	}
	return &apiv1schema.ListIndexesResponse{Indexes: indexes}, nil
}

func (s *Catalog) GetIndex(ctx context.Context, req *apiv1schema.GetIndexRequest) (*apiv1schema.GetIndexResponse, error) {
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	var schemaName, tableName string
	if err := data.QueryRow(ctx, `
		SELECT schemaname, tablename FROM pg_indexes WHERE indexname = $1 LIMIT 1`,
		req.Id,
	).Scan(&schemaName, &tableName); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("index not found")
		}
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	var tableID string
	if err := s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT name FROM lc_tables WHERE tenant_id = $1 AND schema_name = $2 AND name = $3`,
		tid, schemaName, tableName,
	).Scan(&tableID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("index table not found in metadata")
		}
		return nil, err
	}
	pgRows, err := s.ListPGIndexes(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	var match *pgIndexRow
	for i := range pgRows {
		if pgRows[i].Name == req.Id {
			match = &pgRows[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("index not found")
	}
	apiIndexes, err := s.PGIndexesToAPI(ctx, tableID, schemaName, tableName, []pgIndexRow{*match})
	if err != nil {
		return nil, err
	}
	if len(apiIndexes) == 0 {
		return nil, fmt.Errorf("index not found")
	}
	return &apiv1schema.GetIndexResponse{Index: apiIndexes[0]}, nil
}

func (s *Catalog) CreateIndex(ctx context.Context, req *apiv1schema.CreateIndexRequest) (*apiv1schema.CreateIndexResponse, error) {
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	cols, schemaName, tableName, err := s.LoadColumns(ctx, req.TableId)
	if err != nil {
		return nil, err
	}
	resolvedTable, err := s.B.ResolveTableName(ctx, req.TableId)
	if err != nil {
		return nil, err
	}

	colIDSet := make(map[string]struct{}, len(req.ColumnIds))
	for _, id := range req.ColumnIds {
		colIDSet[id] = struct{}{}
	}
	var pgColumns []string
	for _, c := range cols {
		if _, ok := colIDSet[c.Id]; ok {
			pgColumns = append(pgColumns, pgx.Identifier{c.Name}.Sanitize())
		}
	}
	if len(pgColumns) == 0 {
		return nil, fmt.Errorf("no valid columns for index")
	}

	pgIndex, err := indexSQLName(tableName, req.Name)
	if err != nil {
		return nil, err
	}

	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	uniqueSQL := ""
	if req.IsUnique {
		uniqueSQL = "UNIQUE "
	}
	indexSQL := fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS %s ON %s.%s (%s)`,
		uniqueSQL,
		pgx.Identifier{pgIndex}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		strings.Join(pgColumns, ", "),
	)
	if _, err := data.Exec(ctx, indexSQL); err != nil {
		return nil, err
	}

	pgRows, err := s.ListPGIndexes(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	var target *apiv1schema.Index
	apiIndexes, err := s.PGIndexesToAPI(ctx, resolvedTable, schemaName, tableName, pgRows)
	if err != nil {
		return nil, err
	}
	for _, idx := range apiIndexes {
		if idx.PgIndex == pgIndex {
			target = idx
			break
		}
	}
	if target == nil {
		target = &apiv1schema.Index{
			Id:        pgIndex,
			TableId:   resolvedTable,
			Name:      req.Name,
			PgIndex:   pgIndex,
			ColumnIds: req.ColumnIds,
			IsUnique:  req.IsUnique,
		}
	} else {
		target.Name = req.Name
	}
	return &apiv1schema.CreateIndexResponse{Index: target}, nil
}

func (s *Catalog) DeleteIndex(ctx context.Context, req *apiv1schema.DeleteIndexRequest) (*apiv1schema.DeleteIndexResponse, error) {
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	schemaName, err := s.resolveIndexSchema(ctx, req.Id)
	if err != nil {
		return &apiv1schema.DeleteIndexResponse{}, nil
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	drop := fmt.Sprintf(`DROP INDEX IF EXISTS %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{req.Id}.Sanitize(),
	)
	if _, err := data.Exec(ctx, drop); err != nil {
		return nil, err
	}
	return &apiv1schema.DeleteIndexResponse{}, nil
}
