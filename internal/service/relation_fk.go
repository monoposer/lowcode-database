package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/columntype"
)

func validateRollupConfig(cfg map[string]any) error {
	if cfgString(cfg, "relation_column_id") == "" {
		return fmt.Errorf("rollup config requires relation_column_id")
	}
	agg := cfgString(cfg, "aggregate")
	if agg == "" {
		return fmt.Errorf("rollup config requires aggregate (sum|count|min|max|avg)")
	}
	switch agg {
	case "sum", "count", "min", "max", "avg", "SUM", "COUNT", "MIN", "MAX", "AVG":
		return nil
	default:
		return fmt.Errorf("rollup aggregate %q not supported", agg)
	}
}

func (s *LowcodeService) validateRelationFKConfig(ctx context.Context, tenantID string, cfg map[string]any) error {
	targetTable := cfgString(cfg, "target_table_id")
	if targetTable == "" {
		return fmt.Errorf("relation_fk config requires target_table_id")
	}
	if _, err := s.resolveTableName(ctx, targetTable); err != nil {
		return fmt.Errorf("relation_fk target table: %w", err)
	}
	targetColID := cfgString(cfg, "target_column_id")
	if targetColID != "" {
		meta := s.tenants.MetaPool()
		resolved, _ := s.resolveTableName(ctx, targetTable)
		var targetTypeID string
		if err := meta.QueryRow(ctx, `
			SELECT type_id FROM lc_columns
			WHERE id = $1 AND tenant_id = $2 AND table_id = $3`,
			targetColID, tenantID, resolved,
		).Scan(&targetTypeID); err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("relation_fk target_column_id must reference a physical column on target table")
			}
			return err
		}
		if columntype.IsVirtual(targetTypeID) {
			return fmt.Errorf("relation_fk target_column_id must reference a physical column on target table")
		}
	}
	return nil
}

func (s *LowcodeService) addRelationFKConstraint(ctx context.Context, data *pgxpool.Pool, schemaName, tableName, pgColumn string, cfg map[string]any) error {
	if !cfgBool(cfg, "add_fk") {
		return nil
	}
	targetTable := cfgString(cfg, "target_table_id")
	resolved, err := s.resolveTableName(ctx, targetTable)
	if err != nil {
		return err
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return err
	}
	meta := s.tenants.MetaPool()
	var tgtSchema, tgtPhys string
	if err := meta.QueryRow(ctx, `SELECT schema_name, table_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, resolved, tid).
		Scan(&tgtSchema, &tgtPhys); err != nil {
		return err
	}
	targetPgCol := "id"
	if colID := cfgString(cfg, "target_column_id"); colID != "" {
		if err := meta.QueryRow(ctx, `SELECT pg_column FROM lc_columns WHERE id = $1 AND tenant_id = $2`, colID, tid).Scan(&targetPgCol); err != nil {
			return err
		}
	}
	constraintName := "fk_" + pgColumn
	stmt := fmt.Sprintf(`ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s (%s)`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{constraintName}.Sanitize(),
		pgx.Identifier{pgColumn}.Sanitize(),
		pgx.Identifier{tgtSchema}.Sanitize(),
		pgx.Identifier{tgtPhys}.Sanitize(),
		pgx.Identifier{targetPgCol}.Sanitize(),
	)
	_, err = data.Exec(ctx, stmt)
	return err
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func cfgBool(cfg map[string]any, key string) bool {
	if cfg == nil {
		return false
	}
	v, ok := cfg[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	default:
		return false
	}
}
