package service

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/tenant"
)

// -------- shared helpers --------

func (s *LowcodeService) tenantID(ctx context.Context) (string, error) {
	id := strings.TrimSpace(tenant.FromContext(ctx))
	if id == "" {
		return "", fmt.Errorf("X-Tenant-Id is required")
	}
	return id, nil
}

type columnMeta struct {
	Id         string
	TableId    string
	Name       string
	TypeId     string
	PgType     string // 实际 PG 类型，用于写入时空字符串转 NULL
	PgColumn   string
	IsNullable bool
	Position   int32
}

// resolveTableName 接受对外使用的 table 标识（可以是内部 UUID，也可以是逻辑 name），
// 并解析成 lc_tables.name，供以 name 作为外键的表（如 lc_columns）使用。
func (s *LowcodeService) resolveTableName(ctx context.Context, tableIdentifier string) (string, error) {
	if tableIdentifier == "" {
		return "", fmt.Errorf("table_id is required")
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return "", err
	}
	meta := s.tenants.MetaPool()
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

func (s *LowcodeService) loadTablePhysical(ctx context.Context, tableID string) (logicalName, schemaName, tableName string, err error) {
	logicalName, err = s.resolveTableName(ctx, tableID)
	if err != nil {
		return "", "", "", err
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return "", "", "", err
	}
	if err := s.tenants.MetaPool().QueryRow(ctx, `
		SELECT schema_name, table_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`,
		logicalName, tid,
	).Scan(&schemaName, &tableName); err != nil {
		return "", "", "", err
	}
	return logicalName, schemaName, tableName, nil
}

func (s *LowcodeService) loadColumns(ctx context.Context, tableID string) ([]columnMeta, string, string, error) {
	resolvedName, err := s.resolveTableName(ctx, tableID)
	if err != nil {
		return nil, "", "", err
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	meta := s.tenants.MetaPool()
	const q = `
		SELECT c.id, c.table_id, c.name, c.type_id, c.pg_column, c.is_nullable, c.position,
		       t.schema_name, t.table_name
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.table_id = $1 AND c.tenant_id = $2
		ORDER BY c.position
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var cols []columnMeta
	var schemaName, tableName string
	for rows.Next() {
		var c columnMeta
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position, &schemaName, &tableName); err != nil {
			return nil, "", "", err
		}
		if columntype.IsVirtual(c.TypeId) {
			continue
		}
		c.PgType = columntype.PgType(c.TypeId)
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	return cols, schemaName, tableName, nil
}

// relationshipColumn 表示一个 relationship 类型列的元数据，用于 expand 查询。
// Config 约定：target_table_id=关联表 id；link_column_id=子表中外键列 id（一对多）；target_column_id=本表中外键列 id（多对一/一对一）。
// Cardinality: "many" 使用 link 路径；"one" 使用 target 路径。
type relationshipColumn struct {
	Id             string
	TargetTableId  string
	LinkColumnId   string // 子表指向当前表行 id 的列，有则为一对多
	TargetColumnId string // 本表存目标行 id 的列，有则为多对一/一对一
	Cardinality    string // "one" | "many"
}

// cfgString reads a string-ish config value.
func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// NormalizeRelationshipConfig validates relationship column config, enforces link vs target
// exclusivity, and sets cardinality for persistence.
func NormalizeRelationshipConfig(cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	out := maps.Clone(cfg)
	targetTable := cfgString(out, "target_table_id")
	if targetTable == "" {
		return nil, fmt.Errorf("relationship config requires target_table_id")
	}
	linkID := cfgString(out, "link_column_id")
	targetColID := cfgString(out, "target_column_id")
	card := strings.ToLower(cfgString(out, "cardinality"))

	if linkID != "" && targetColID != "" {
		return nil, fmt.Errorf("relationship config: set only one of link_column_id (many) or target_column_id (one), not both")
	}
	if linkID == "" && targetColID == "" {
		return nil, fmt.Errorf("relationship config requires link_column_id (many) or target_column_id (one)")
	}
	if linkID != "" {
		if card == "one" {
			return nil, fmt.Errorf("relationship cardinality one requires target_column_id, not link_column_id")
		}
		out["cardinality"] = "many"
		delete(out, "target_column_id")
		return out, nil
	}
	// targetColID only
	if card == "many" {
		return nil, fmt.Errorf("relationship cardinality many requires link_column_id")
	}
	out["cardinality"] = "one"
	delete(out, "link_column_id")
	return out, nil
}

func effectiveRelationshipCardinality(cfg map[string]any, linkID, targetColID string) string {
	if c := strings.ToLower(cfgString(cfg, "cardinality")); c == "one" || c == "many" {
		return c
	}
	if linkID != "" && targetColID != "" {
		return "many"
	}
	if linkID != "" {
		return "many"
	}
	return "one"
}

// loadRelationshipColumns 加载表中指定 id 的 relationship 列及其 config。
func (s *LowcodeService) loadRelationshipColumns(ctx context.Context, tableID string, columnIDs []string) ([]relationshipColumn, error) {
	if len(columnIDs) == 0 {
		return nil, nil
	}
	resolvedName, err := s.resolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	placeholders := make([]string, len(columnIDs))
	args := make([]any, 0, 2+len(columnIDs))
	args = append(args, resolvedName, tid)
	for i := range columnIDs {
		placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
		args = append(args, columnIDs[i])
	}
	q := fmt.Sprintf(`
		SELECT c.id, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship' AND c.id IN (%s)
	`, strings.Join(placeholders, ", "))
	rows, err := meta.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []relationshipColumn
	for rows.Next() {
		var id string
		var cfg map[string]any
		if err := rows.Scan(&id, &cfg); err != nil {
			return nil, err
		}
		rc := relationshipColumn{Id: id}
		if cfg != nil {
			if v, _ := cfg["target_table_id"].(string); v != "" {
				rc.TargetTableId = v
			}
			if v, _ := cfg["link_column_id"].(string); v != "" {
				rc.LinkColumnId = v
			}
			if v, _ := cfg["target_column_id"].(string); v != "" {
				rc.TargetColumnId = v
			}
		}
		if rc.TargetTableId == "" {
			continue
		}
		if rc.LinkColumnId == "" && rc.TargetColumnId == "" {
			continue
		}
		rc.Cardinality = effectiveRelationshipCardinality(cfg, rc.LinkColumnId, rc.TargetColumnId)
		out = append(out, rc)
	}
	return out, rows.Err()
}

// valueToAny 把 API Value 转成可以写入 PG 的 Go 值。
func valueToAny(v *apiv1.Value) any {
	return valueToAnyForColumn(v, "")
}

// valueToAnyForColumn 根据列 PG 类型转换：空字符串写入 numeric/timestamptz/boolean 等时转为 nil (NULL)。
func valueToAnyForColumn(v *apiv1.Value, pgType string) any {
	raw := valueToAnyRaw(v)
	// 空字符串且列为非 text 类型时写 NULL，避免 "invalid input syntax for type numeric: \"\""
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
		if m, ok := v.JsonValue.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// anyToValue 把 PG 返回的值转成 API Value，简单处理常见类型。
// PG numeric/int 等可能被 pgx 扫成 pgtype.Numeric，需转成 float64 再返回，否则会变成 "{69 -1 false finite true}" 这种 struct 字符串。
func anyToValue(v any) *apiv1.Value {
	switch t := v.(type) {
	case string:
		return apiv1.StringValue(t)
	case []byte:
		return apiv1.BytesValue(t)
	case bool:
		return apiv1.BoolValue(t)
	case time.Time:
		return apiv1.TimestampValue(t)
	case int32, int64, float32, float64:
		f := toFloat64(t)
		return apiv1.NumberValue(f)
	case pgtype.Numeric:
		if f, err := numericToFloat64(t); err == nil {
			return apiv1.NumberValue(f)
		}
	case *pgtype.Numeric:
		if t != nil {
			if f, err := numericToFloat64(*t); err == nil {
				return apiv1.NumberValue(f)
			}
		}
	default:
		return apiv1.StringValue(fmt.Sprint(v))
	}
	return apiv1.StringValue(fmt.Sprint(v))
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

// RowToMap encodes a Row as JSON-compatible map (for webhooks / logging).
func RowToMap(r *apiv1.Row) map[string]any {
	if r == nil {
		return nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return map[string]any{"id": r.Id}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"id": r.Id}
	}
	return m
}
