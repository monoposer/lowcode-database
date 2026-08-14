package shared

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/monoposer/lowcode-database/pkg/tenant"
	"strings"
)

// -------- Value --------

func ValueToAny(v *Value) any {
	return ValueToAnyForColumn(v, "")
}

func ValueToAnyForColumn(v *Value, pgType string) any {
	raw := valueToAnyRaw(v)
	if s, ok := raw.(string); ok && s == "" && pgType != "" && pgType != "text" && pgType != "jsonb" && pgType != "json" {
		return nil
	}
	return raw
}

func valueToAnyRaw(v *Value) any {
	if v == nil {
		return nil
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.NumberValue != nil {
		return *v.NumberValue
	}
	if v.BoolValue != nil {
		return *v.BoolValue
	}
	if v.TimestampValue != nil {
		return *v.TimestampValue
	}
	if v.BytesValue != nil {
		return v.BytesValue
	}
	if v.JsonValue != nil {
		return v.JsonValue
	}
	return nil
}

func AnyToValue(v any) *Value {
	return DBCellValue(v, "")
}

func numericToFloat64(n pgtype.Numeric) (float64, error) {
	f8, err := n.Float64Value()
	if err != nil || !f8.Valid {
		return 0, fmt.Errorf("invalid numeric")
	}
	return f8.Float64, nil
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case float32:
		return float64(t)
	case float64:
		return t
	default:
		return 0
	}
}

// -------- Tenant / Base --------

func (b *Base) TenantID(ctx context.Context) (string, error) {
	if b.Tenants != nil {
		return b.Tenants.ResolveTenantID(ctx)
	}
	id := tenant.ResolveTenantID(ctx)
	if id == "" {
		return "", fmt.Errorf("X-Tenant-Id is required")
	}
	return id, nil
}

// BaseID returns X-Base-Id or the tenant default base.
func (b *Base) BaseID(ctx context.Context) (string, error) {
	tenantID, err := b.TenantID(ctx)
	if err != nil {
		return "", err
	}
	if b.Tenants == nil {
		if base := strings.TrimSpace(tenant.BaseFromContext(ctx)); base != "" {
			return base, nil
		}
		return "", fmt.Errorf("X-Base-Id is required")
	}
	return b.Tenants.ResolveBaseID(ctx, tenantID)
}

func (b *Base) ResolveTableName(ctx context.Context, tableRef string) (string, error) {
	if tableRef == "" {
		return "", fmt.Errorf("table_name is required")
	}
	tenantID, err := b.TenantID(ctx)
	if err != nil {
		return "", err
	}
	baseID, err := b.BaseID(ctx)
	if err != nil {
		return "", err
	}
	meta := b.Tenants.MetaPool()
	const q = `
		SELECT name
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2 AND base_id = $3
	`
	var name string
	if err := meta.QueryRow(ctx, q, tableRef, tenantID, baseID).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func (b *Base) LoadTablePhysical(ctx context.Context, tableName string) (logicalName, schemaName, physicalName string, err error) {
	logicalName, err = b.ResolveTableName(ctx, tableName)
	if err != nil {
		return "", "", "", err
	}
	return logicalName, "", logicalName, nil
}

// CellByRef reads a cell value keyed by logical column name.
func CellByRef(cells map[string]*Value, c ColumnMeta) (*Value, bool) {
	if cells == nil {
		return nil, false
	}
	if v, ok := cells[c.Name]; ok {
		return v, true
	}
	return nil, false
}

// CellsToNames re-keys a cell map to logical column names for API responses.
func CellsToNames(cells map[string]*Value, cols []ColumnMeta) map[string]*Value {
	if len(cells) == 0 {
		return cells
	}
	out := make(map[string]*Value, len(cells))
	for _, c := range cols {
		if v, ok := CellByRef(cells, c); ok {
			out[c.Name] = v
		}
	}
	for k, v := range cells {
		if _, ok := out[k]; ok {
			continue
		}
		out[k] = v
	}
	return out
}

// NormalizeInputCells keeps cells keyed by column name (unknown keys preserved).
func NormalizeInputCells(cells map[string]*Value, cols []ColumnMeta) map[string]*Value {
	if len(cells) == 0 {
		return cells
	}
	byName := make(map[string]ColumnMeta, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	out := make(map[string]*Value, len(cells))
	for key, v := range cells {
		if key == "" {
			continue
		}
		if c, ok := byName[key]; ok {
			out[c.Name] = v
			continue
		}
		out[key] = v
	}
	return out
}
