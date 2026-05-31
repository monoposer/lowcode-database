package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type pgColumnQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PhysicalColumnPgType reads the PostgreSQL type of a physical column from information_schema.
func PhysicalColumnPgType(ctx context.Context, db pgColumnQuerier, schemaName, tableName, columnName string) (string, error) {
	const sql = `
		SELECT data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
	`
	var dataType, udtName string
	err := db.QueryRow(ctx, sql, schemaName, tableName, columnName).Scan(&dataType, &udtName)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("column %s.%s.%s not found", schemaName, tableName, columnName)
	}
	if err != nil {
		return "", err
	}
	if strings.EqualFold(dataType, "USER-DEFINED") && udtName != "" {
		return udtName, nil
	}
	return dataType, nil
}

// RelationFKColumnPgType returns the PG type for a relation_fk column matching its target reference.
func (b *Base) RelationFKColumnPgType(ctx context.Context, tenantID string, cfg map[string]any) (string, error) {
	targetTable := CfgString(cfg, "target_table_id")
	if targetTable == "" {
		return "", fmt.Errorf("relation_fk config requires target_table_id")
	}
	resolved, err := b.ResolveTableName(ctx, targetTable)
	if err != nil {
		return "", err
	}
	targetCol := "id"
	if ref := CfgString(cfg, "target_column_id"); ref != "" {
		targetCol = ref
	}
	var schemaName string
	if err := b.Tenants.MetaPool().QueryRow(ctx, `
		SELECT schema_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`,
		resolved, tenantID,
	).Scan(&schemaName); err != nil {
		return "", err
	}
	data, err := b.Tenants.DataPool(ctx)
	if err != nil {
		return "", err
	}
	return PhysicalColumnPgType(ctx, data, schemaName, resolved, targetCol)
}
