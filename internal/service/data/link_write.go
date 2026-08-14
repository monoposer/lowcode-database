package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Data) persistLinks(ctx context.Context, pool *pgxpool.Pool, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := persistLinksTx(ctx, tx, tenantID, tableName, recordID, cols, links); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func persistLinksTx(ctx context.Context, tx pgx.Tx, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) error {
	for fieldRef, ids := range links {
		col, ok := linkFieldMeta(cols, fieldRef)
		if !ok {
			continue
		}
		toTable := shared.CfgString(col.Config, "to_table_name")
		if toTable == "" {
			toTable = shared.CfgString(col.Config, "target_table_name")
		}
		fieldKey := col.Name
		oldIDs, _ := calc.ListToIDs(ctx, tx, tenantID, recordID, fieldKey)
		if err := calc.ReplaceLinks(ctx, tx, tenantID, tableName, recordID, fieldKey, toTable, ids); err != nil {
			return err
		}
		if shared.CfgBool(col.Config, "bidirectional") {
			inv := shared.CfgString(col.Config, "inverse_field_id")
			if inv == "" {
				inv = shared.CfgString(col.Config, "inverse_field_name")
			}
			if err := calc.DeleteInverseForField(ctx, tx, tenantID, inv, oldIDs, recordID); err != nil {
				return err
			}
			for _, toID := range ids {
				if err := calc.InsertInverse(ctx, tx, tenantID, tableName, recordID, inv, toTable, toID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
