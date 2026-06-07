package shared

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/monoposer/lowcode-database/internal/event"
)

// EmitEvent records metadata.* changes to lc_schema_audit and publishes
// records.* / schema.* (metadata.* aliased) onto EventBus.
func (b *Base) EmitEvent(ctx context.Context, eventType, tableID string, data map[string]any) {
	if b == nil {
		return
	}
	if strings.HasPrefix(eventType, "metadata.") {
		b.recordSchemaAudit(ctx, eventType, tableID, data)
	}
	if b.EventBus == nil {
		return
	}
	tid, _ := b.TenantID(ctx)
	env := event.Envelope{
		ID:       uuid.NewString(),
		Type:     event.BusType(eventType),
		TenantID: tid,
		TableID:  tableID,
		Data:     data,
		Time:     time.Now().UTC(),
	}
	_ = b.EventBus.Publish(ctx, env)
}

// RecordDDL writes the original DDL SQL onto lc_schema_audit.detail.ddl.
func (b *Base) RecordDDL(ctx context.Context, action, tableID, ddl string, extra map[string]any) {
	if b == nil {
		return
	}
	detail := map[string]any{}
	for k, v := range extra {
		detail[k] = v
	}
	if ddl != "" {
		detail["ddl"] = ddl
	}
	b.recordSchemaAudit(ctx, action, tableID, detail)
}

func (b *Base) recordSchemaAudit(ctx context.Context, eventType, tableID string, data map[string]any) {
	if b == nil || b.Tenants == nil {
		return
	}
	tid, err := b.TenantID(ctx)
	if err != nil {
		return
	}
	resourceType, resourceID := schemaAuditResource(eventType, data)
	detail := data
	if detail == nil {
		detail = map[string]any{}
	}
	raw, _ := json.Marshal(detail)
	_, _ = b.Tenants.MetaPool().Exec(ctx, `
		INSERT INTO lc_schema_audit (tenant_id, action, resource_type, resource_id, table_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
	`, tid, eventType, resourceType, resourceID, tableID, string(raw))
}

func schemaAuditResource(eventType string, data map[string]any) (resourceType, resourceID string) {
	switch eventType {
	case event.MetadataTableCreated, event.MetadataTableDeleted, event.MetadataTableRenamed:
		return "table", tableIDFromData(data, "tableId", "table")
	case event.MetadataColumnCreated, event.MetadataColumnUpdated, event.MetadataColumnDeleted:
		return "column", nestedID(data, "column", "id", "name")
	case event.MetadataRelationCreated, event.MetadataRelationDeleted:
		return "relation", nestedID(data, "relation", "name", "id")
	case event.MetadataIndexCreated, event.MetadataIndexDeleted:
		return "index", nestedID(data, "index", "id", "name")
	case event.MetadataQueryCreated, event.MetadataQueryUpdated, event.MetadataQueryDeleted:
		return "query", nestedID(data, "query", "name", "id")
	default:
		return "metadata", ""
	}
}

func tableIDFromData(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func nestedID(data map[string]any, objKey string, fields ...string) string {
	obj, _ := data[objKey].(map[string]any)
	if obj == nil {
		return tableIDFromData(data, fields...)
	}
	for _, f := range fields {
		if v, ok := obj[f].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// TouchUpdatedAtSQL appends updated_at = now() to UPDATE SET clauses.
func TouchUpdatedAtSQL(setParts []string) []string {
	for _, p := range setParts {
		if p == "updated_at = now()" {
			return setParts
		}
	}
	return append(setParts, "updated_at = now()")
}
