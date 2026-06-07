package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// CreateRow / UpdateRow / DeleteRow live in vr_rows.go (virtual_records is the only store).

// ListRows delegates to QueryRows for filter/sort/pagination support.
func (s *Data) ListRows(ctx context.Context, req *row.ListRowsRequest) (*row.ListRowsResponse, error) {
	qresp, err := s.QueryRows(ctx, &row.QueryRowsRequest{
		TableId:     req.TableId,
		PageSize:    req.PageSize,
		PageToken:   req.PageToken,
		Consistency: req.Consistency,
	})
	if err != nil {
		return nil, err
	}
	return &row.ListRowsResponse{
		Rows:          qresp.Rows,
		NextPageToken: qresp.NextPageToken,
	}, nil
}

// insertRowTx inserts one row; cells keys are logical column names (legacy UUID accepted).
func (s *Data) insertRowTx(ctx context.Context, tx pgx.Tx, cols []shared.ColumnMeta, schemaName, tableName string, cells map[string]*apiv1.Value) (string, error) {
	cells = shared.NormalizeInputCells(cells, cols)
	var pgCols []string
	var args []any
	for _, c := range cols {
		val, ok := cells[c.Name]
		if !ok {
			continue
		}
		pgCols = append(pgCols, c.Name)
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
	}
	if len(pgCols) == 0 {
		return "", nil
	}
	colsSQL := strings.Join(pgCols, ", ")
	params := make([]string, len(pgCols))
	for i := range params {
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	paramSQL := strings.Join(params, ", ")
	insert := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s) RETURNING id::text`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		colsSQL, paramSQL)
	var id string
	if err := tx.QueryRow(ctx, insert, args...).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Data) updateRowTx(ctx context.Context, tx pgx.Tx, cols []shared.ColumnMeta, schemaName, tableName, rowID string, cells map[string]*apiv1.Value) error {
	cells = shared.NormalizeInputCells(cells, cols)
	var setParts []string
	var args []any
	argIdx := 1
	for _, c := range cols {
		val, ok := cells[c.Name]
		if !ok {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", pgx.Identifier{c.Name}.Sanitize(), argIdx))
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
		argIdx++
	}
	if len(setParts) == 0 {
		return nil
	}
	setParts = shared.TouchUpdatedAtSQL(setParts)
	args = append(args, rowID)
	update := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE id = $%d`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		strings.Join(setParts, ", "),
		argIdx,
	)
	_, err := tx.Exec(ctx, update, args...)
	return err
}
