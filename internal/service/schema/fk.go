package schema

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/service/shared"
)

func (s *Schema) ValidateRelationFKConfig(ctx context.Context, tenantID string, cfg map[string]any) error {
	targetTable := shared.CfgString(cfg, "target_table_id")
	if targetTable == "" {
		return fmt.Errorf("relation_fk config requires target_table_id")
	}
	if _, err := s.B.ResolveTableName(ctx, targetTable); err != nil {
		return fmt.Errorf("relation_fk target table: %w", err)
	}
	targetColRef := shared.CfgString(cfg, "target_column_id")
	if targetColRef != "" {
		meta := s.B.Tenants.MetaPool()
		resolved, _ := s.B.ResolveTableName(ctx, targetTable)
		var targetTypeID string
		if err := meta.QueryRow(ctx, `
			SELECT type_id FROM lc_columns
			WHERE name = $1 AND tenant_id = $2 AND table_id = $3`,
			targetColRef, tenantID, resolved,
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

func (s *Schema) AddRelationFKConstraint(ctx context.Context, data *pgxpool.Pool, schemaName, tableName, colName string, cfg map[string]any) error {
	if !shared.CfgBool(cfg, "add_fk") {
		return nil
	}
	targetTable := shared.CfgString(cfg, "target_table_id")
	resolved, err := s.B.ResolveTableName(ctx, targetTable)
	if err != nil {
		return err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()
	var tgtSchema string
	if err := meta.QueryRow(ctx, `SELECT schema_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, resolved, tid).
		Scan(&tgtSchema); err != nil {
		return err
	}
	targetCol := "id"
	if colRef := shared.CfgString(cfg, "target_column_id"); colRef != "" {
		if targetCol, err = s.ResolveColumnName(ctx, tid, resolved, colRef); err != nil {
			return err
		}
	}
	constraintName := "fk_" + colName
	stmt := fmt.Sprintf(`ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s (%s)`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{constraintName}.Sanitize(),
		pgx.Identifier{colName}.Sanitize(),
		pgx.Identifier{tgtSchema}.Sanitize(),
		pgx.Identifier{resolved}.Sanitize(),
		pgx.Identifier{targetCol}.Sanitize(),
	)
	_, err = data.Exec(ctx, stmt)
	return err
}
