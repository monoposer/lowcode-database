package shared

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/monoposer/lowcode-database/internal/event"
)

// EmitEvent delivers a domain event to subscribed sinks when configured.
// metadata.* events are also persisted to lc_schema_audit.
func (b *Base) EmitEvent(ctx context.Context, eventType, tableID string, data map[string]any) {
	if b == nil {
		return
	}
	if strings.HasPrefix(eventType, "metadata.") {
		b.recordSchemaAudit(ctx, eventType, tableID, data)
	}
	if b.Events == nil {
		return
	}
	b.Events.Emit(ctx, eventType, tableID, data)
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
	case event.MetadataChoiceCreated, event.MetadataChoiceUpdated, event.MetadataChoiceDeleted:
		return "choice", nestedID(data, "choice", "id", "name")
	case event.MetadataRelationCreated, event.MetadataRelationDeleted:
		return "relation", nestedID(data, "relation", "name", "id")
	case event.MetadataIndexCreated, event.MetadataIndexDeleted:
		return "index", nestedID(data, "index", "id", "name")
	case event.MetadataDataSourceCreated, event.MetadataDataSourceUpdated, event.MetadataDataSourceDeleted:
		return "data_source", nestedID(data, "dataSource", "name", "id")
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
