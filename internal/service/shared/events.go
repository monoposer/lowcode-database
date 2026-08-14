package shared

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/monoposer/lowcode-database/internal/event"
)

// EmitEvent publishes records.* / schema.* (metadata.* aliased) onto EventBus.
// This service does not persist an audit log; consumers use webhooks or the bus.
func (b *Base) EmitEvent(ctx context.Context, eventType, tableName string, data map[string]any) {
	if b == nil || b.EventBus == nil {
		return
	}
	tid, _ := b.TenantID(ctx)
	env := event.Envelope{
		ID:       uuid.NewString(),
		Type:     event.BusType(eventType),
		TenantID: tid,
		TableName:  tableName,
		Data:     data,
		Time:     time.Now().UTC(),
	}
	_ = b.EventBus.Publish(ctx, env)
}

// TouchUpdatedAtSQL appends updated_at = now() to UPDATE SET clauses.
func (b *Base) TouchUpdatedAtSQL(setParts []string) []string {
	for _, p := range setParts {
		if p == "updated_at = now()" {
			return setParts
		}
	}
	return append(setParts, "updated_at = now()")
}
