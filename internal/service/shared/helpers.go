package shared

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/tenant"
	"strings"
)

// -------- Value --------
// -------- Value --------

func ValueToAny(v *apiv1.Value) any {
	return ValueToAnyForColumn(v, "")
}

func ValueToAnyForColumn(v *apiv1.Value, pgType string) any {
	raw := valueToAnyRaw(v)
	if s, ok := raw.(string); ok && s == "" && pgType != "" && pgType != "text" && pgType != "jsonb" && pgType != "json" {
		return nil
	}
	return raw
}

func valueToAnyRaw(v *apiv1.Value) any {
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

func AnyToValue(v any) *apiv1.Value {
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

func RowToMap(r *row.Row) map[string]any {
	if r == nil {
		return nil
	}
	m := map[string]any{"id": r.Id}
	for k, v := range r.Cells {
		m[k] = apiv1.ValueToNative(v)
	}
	return m
}

// -------- Tenant --------

func (b *Base) TenantID(ctx context.Context) (string, error) {
	id := strings.TrimSpace(tenant.FromContext(ctx))
	if id == "" {
		return "", fmt.Errorf("X-Tenant-Id is required")
	}
	return id, nil
}

func (b *Base) ResolveTableName(ctx context.Context, tableIdentifier string) (string, error) {
	if tableIdentifier == "" {
		return "", fmt.Errorf("table_id is required")
	}
	tid, err := b.TenantID(ctx)
	if err != nil {
		return "", err
	}
	meta := b.Tenants.MetaPool()
	const q = `
		SELECT name
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2
	`
	var name string
	if err := meta.QueryRow(ctx, q, tableIdentifier, tid).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func (b *Base) LoadTablePhysical(ctx context.Context, tableID string) (logicalName, schemaName, tableName string, err error) {
	logicalName, err = b.ResolveTableName(ctx, tableID)
	if err != nil {
		return "", "", "", err
	}
	tid, err := b.TenantID(ctx)
	if err != nil {
		return "", "", "", err
	}
	if err := b.Tenants.MetaPool().QueryRow(ctx, `
		SELECT schema_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`,
		logicalName, tid,
	).Scan(&schemaName); err != nil {
		return "", "", "", err
	}
	return logicalName, schemaName, logicalName, nil
}

// CellByRef reads a cell value keyed by logical column name (preferred) or legacy meta UUID.
func CellByRef(cells map[string]*apiv1.Value, c ColumnMeta) (*apiv1.Value, bool) {
	if cells == nil {
		return nil, false
	}
	if v, ok := cells[c.Name]; ok {
		return v, true
	}
	if c.Id != "" && c.Id != c.Name {
		if v, ok := cells[c.Id]; ok {
			return v, true
		}
	}
	return nil, false
}

// CellsToNames re-keys a cell map to logical column names for API responses.
func CellsToNames(cells map[string]*apiv1.Value, cols []ColumnMeta) map[string]*apiv1.Value {
	if len(cells) == 0 {
		return cells
	}
	out := make(map[string]*apiv1.Value, len(cells))
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

// NormalizeInputCells accepts cells keyed by column name or legacy UUID and returns name-keyed cells.
func NormalizeInputCells(cells map[string]*apiv1.Value, cols []ColumnMeta) map[string]*apiv1.Value {
	if len(cells) == 0 {
		return cells
	}
	byID := make(map[string]ColumnMeta, len(cols))
	byName := make(map[string]ColumnMeta, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
		if c.Id != "" {
			byID[c.Id] = c
		}
	}
	out := make(map[string]*apiv1.Value, len(cells))
	for key, v := range cells {
		if key == "" {
			continue
		}
		if c, ok := byName[key]; ok {
			out[c.Name] = v
			continue
		}
		if c, ok := byID[key]; ok {
			out[c.Name] = v
			continue
		}
		out[key] = v
	}
	return out
}
