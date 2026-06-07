package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/monoposer/lowcode-database/internal/apiv1/platform"
)

func (s *Platform) ListSchemaAudit(ctx context.Context, req *platform.ListSchemaAuditRequest) (*platform.ListSchemaAuditResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close()
			Err() error
		}
		errQuery error
	)
	if req.TableId != "" {
		rows, errQuery = s.B.Tenants.MetaPool().Query(ctx, `
			SELECT id, action, resource_type, resource_id, table_id, detail, occurred_at
			FROM lc_schema_audit
			WHERE tenant_id = $1 AND table_id = $2
			ORDER BY occurred_at DESC
			LIMIT $3
		`, tid, req.TableId, limit)
	} else {
		rows, errQuery = s.B.Tenants.MetaPool().Query(ctx, `
			SELECT id, action, resource_type, resource_id, table_id, detail, occurred_at
			FROM lc_schema_audit
			WHERE tenant_id = $1
			ORDER BY occurred_at DESC
			LIMIT $2
		`, tid, limit)
	}
	if errQuery != nil {
		return nil, errQuery
	}
	defer rows.Close()
	var out platform.ListSchemaAuditResponse
	for rows.Next() {
		e, err := scanSchemaAudit(rows)
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, e)
	}
	return &out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSchemaAudit(row rowScanner) (*platform.SchemaAuditEntry, error) {
	var e platform.SchemaAuditEntry
	var detailRaw []byte
	if err := row.Scan(&e.Id, &e.Action, &e.ResourceType, &e.ResourceId, &e.TableId, &detailRaw, &e.OccurredAt); err != nil {
		return nil, fmt.Errorf("scan schema audit: %w", err)
	}
	_ = json.Unmarshal(detailRaw, &e.Detail)
	return &e, nil
}
