package shared

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/monoposer/lowcode-database/internal/apiv1"
	"strings"
	"time"
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
// Under virtual_records there is no physical column; defaults to uuid for id, else text.
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
	baseID, err := b.BaseID(ctx)
	if err != nil {
		return "", err
	}
	var typeID string
	if err := b.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id FROM lc_columns
		WHERE name = $1 AND tenant_id = $2 AND base_id = $3 AND table_id = $4`,
		targetCol, tenantID, baseID, resolved,
	).Scan(&typeID); err != nil {
		if targetCol == "id" {
			return "uuid", nil
		}
		return "", err
	}
	if typeID == "uuid" || typeID == "int8" || typeID == "text" {
		return typeID, nil
	}
	return "text", nil
}

func isUUIDPgType(pgType string) bool {
	pgType = strings.ToLower(strings.TrimSpace(pgType))
	if pgType == "uuid" {
		return true
	}
	if i := strings.LastIndex(pgType, "."); i >= 0 {
		return strings.ToLower(pgType[i+1:]) == "uuid"
	}
	return false
}

func isByteaPgType(pgType string) bool {
	pgType = strings.ToLower(strings.TrimSpace(pgType))
	return pgType == "bytea"
}

// FormatUUID converts pgx UUID wire forms to canonical string.
func FormatUUID(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "", false
		}
		if u, err := uuid.Parse(t); err == nil {
			return u.String(), true
		}
		return t, true
	case []byte:
		if len(t) == 16 {
			u, err := uuid.FromBytes(t)
			if err == nil {
				return u.String(), true
			}
		}
	case [16]byte:
		u, err := uuid.FromBytes(t[:])
		if err == nil {
			return u.String(), true
		}
	case pgtype.UUID:
		if !t.Valid {
			return "", false
		}
		u, err := uuid.FromBytes(t.Bytes[:])
		if err == nil {
			return u.String(), true
		}
	case *pgtype.UUID:
		if t == nil || !t.Valid {
			return "", false
		}
		u, err := uuid.FromBytes(t.Bytes[:])
		if err == nil {
			return u.String(), true
		}
	}
	return "", false
}

// PGValueToNative converts a PostgreSQL driver value to plain JSON scalars.
func PGValueToNative(v any, pgType string) any {
	if v == nil {
		return nil
	}
	if isUUIDPgType(pgType) {
		if s, ok := FormatUUID(v); ok {
			return s
		}
	}
	if isByteaPgType(pgType) {
		switch t := v.(type) {
		case []byte:
			return base64.StdEncoding.EncodeToString(t)
		}
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		if len(t) == 16 {
			if s, ok := FormatUUID(t); ok {
				return s
			}
		}
		return base64.StdEncoding.EncodeToString(t)
	case bool:
		return t
	case time.Time:
		return t.Format(time.RFC3339Nano)
	case int32, int64, float32, float64:
		return toFloat64(t)
	case pgtype.Numeric:
		if f, err := numericToFloat64(t); err == nil {
			return f
		}
	case *pgtype.Numeric:
		if t != nil {
			if f, err := numericToFloat64(*t); err == nil {
				return f
			}
		}
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	case []any:
		return t
	default:
		return fmt.Sprint(v)
	}
	return fmt.Sprint(v)
}

// DBCellValue converts a scanned PG value into an API cell Value.
func DBCellValue(v any, pgType string) *apiv1.Value {
	return apiv1.NativeToValue(PGValueToNative(v, pgType))
}
