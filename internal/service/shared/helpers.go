package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/tenant"
)

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

func RowToMap(r *apiv1.Row) map[string]any {
	if r == nil {
		return nil
	}
	m := map[string]any{"id": r.Id}
	for k, v := range r.Cells {
		m[k] = apiv1.ValueToNative(v)
	}
	return m
}
