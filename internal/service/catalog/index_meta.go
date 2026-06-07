package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
)

type indexMetaRow struct {
	TableID   string
	Name      string
	PgIndex   string
	ColumnIDs []string
	IsUnique  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Catalog) insertIndexMeta(ctx context.Context, tid, baseID, tableID, name, pgIndex string, columnIDs []string, isUnique bool) error {
	colJSON, err := json.Marshal(columnIDs)
	if err != nil {
		return err
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		INSERT INTO lc_indexes (tenant_id, base_id, table_id, name, pg_index, column_ids, is_unique)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
		tid, baseID, tableID, name, pgIndex, colJSON, isUnique,
	)
	return err
}

func (s *Catalog) insertVRIndexMeta(ctx context.Context, tid, baseID, tableID, name, pgIndex, vtID, indexExpr, indexType string, columnIDs []string, isUnique bool) error {
	colJSON, err := json.Marshal(columnIDs)
	if err != nil {
		return err
	}
	if indexType == "" {
		indexType = "btree"
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		INSERT INTO lc_indexes (
			tenant_id, base_id, table_id, name, pg_index, column_ids, is_unique,
			vt_id, index_expr, index_type, migrate_status
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, 'pending')`,
		tid, baseID, tableID, name, pgIndex, colJSON, isUnique, vtID, indexExpr, indexType,
	)
	return err
}

func (s *Catalog) deleteIndexMeta(ctx context.Context, tid, baseID, tableID, name string) error {
	_, err := s.B.Tenants.MetaPool().Exec(ctx, `
		DELETE FROM lc_indexes WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND name = $4`,
		tid, baseID, tableID, name,
	)
	return err
}

func (s *Catalog) listIndexMeta(ctx context.Context, tid, baseID, tableID string) ([]indexMetaRow, error) {
	rows, err := s.B.Tenants.MetaPool().Query(ctx, `
		SELECT table_id, name, pg_index, column_ids, is_unique, created_at, updated_at
		FROM lc_indexes
		WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3
		ORDER BY name`, tid, baseID, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndexMetaRows(rows)
}

func (s *Catalog) getIndexMeta(ctx context.Context, tid, baseID, tableID, name string) (*indexMetaRow, error) {
	row := s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT table_id, name, pg_index, column_ids, is_unique, created_at, updated_at
		FROM lc_indexes
		WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND name = $4`,
		tid, baseID, tableID, name,
	)
	r, err := scanIndexMetaRow(row)
	if err != nil {
		return nil, fmt.Errorf("index %q not found: %w", name, err)
	}
	return r, nil
}

func (s *Catalog) findIndexMeta(ctx context.Context, tid, baseID, tableID, ref string) (*indexMetaRow, error) {
	if tableID != "" {
		if r, err := s.getIndexMeta(ctx, tid, baseID, tableID, ref); err == nil {
			return r, nil
		}
		row := s.B.Tenants.MetaPool().QueryRow(ctx, `
			SELECT table_id, name, pg_index, column_ids, is_unique, created_at, updated_at
			FROM lc_indexes
			WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND pg_index = $4`,
			tid, baseID, tableID, ref,
		)
		if r, err := scanIndexMetaRow(row); err == nil {
			return r, nil
		}
	}
	rows, err := s.B.Tenants.MetaPool().Query(ctx, `
		SELECT table_id, name, pg_index, column_ids, is_unique, created_at, updated_at
		FROM lc_indexes
		WHERE tenant_id = $1 AND base_id = $2 AND (name = $3 OR pg_index = $3)
		ORDER BY name`, tid, baseID, ref)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanIndexMetaRows(rows)
	if err != nil {
		return nil, err
	}
	switch len(list) {
	case 0:
		return nil, fmt.Errorf("index %q not found", ref)
	case 1:
		return &list[0], nil
	default:
		return nil, fmt.Errorf("index %q is ambiguous; pass table_id query parameter", ref)
	}
}

func (s *Catalog) indexMetaToAPI(r indexMetaRow) *apiv1schema.Index {
	return &apiv1schema.Index{
		Id:        r.Name,
		TableId:   r.TableID,
		Name:      r.Name,
		PgIndex:   r.PgIndex,
		ColumnIds: append([]string(nil), r.ColumnIDs...),
		IsUnique:  r.IsUnique,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

type indexMetaScanner interface {
	Scan(dest ...any) error
}

func scanIndexMetaRow(row indexMetaScanner) (*indexMetaRow, error) {
	var r indexMetaRow
	var colRaw []byte
	if err := row.Scan(&r.TableID, &r.Name, &r.PgIndex, &colRaw, &r.IsUnique, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	if len(colRaw) > 0 {
		_ = json.Unmarshal(colRaw, &r.ColumnIDs)
	}
	return &r, nil
}

func scanIndexMetaRows(rows pgx.Rows) ([]indexMetaRow, error) {
	var out []indexMetaRow
	for rows.Next() {
		r, err := scanIndexMetaRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// BackfillIndexesFromPG registers platform-style PG indexes into lc_indexes (idempotent).
// No-op under virtual_records (always on).
func (s *Catalog) BackfillIndexesFromPG(ctx context.Context, _ *apiv1schema.BackfillIndexesRequest) (*apiv1schema.BackfillIndexesResponse, error) {
	n, err := s.backfillIndexesFromPG(ctx)
	if err != nil {
		return nil, err
	}
	return &apiv1schema.BackfillIndexesResponse{IndexesCreated: n}, nil
}

func (s *Catalog) backfillIndexesFromPG(ctx context.Context) (int, error) {
	if s.B.IsRLSTableMode() {
		return 0, nil
	}
	return 0, nil
}

func (s *Catalog) BackfillIndexStatus(ctx context.Context) (*apiv1schema.BackfillIndexStatusResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.B.Tenants.MetaPool().Query(ctx, `
		SELECT migrate_status, COUNT(*)::int
		FROM lc_indexes
		WHERE tenant_id = $1 AND base_id = $2
		GROUP BY migrate_status`, tid, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &apiv1schema.BackfillIndexStatusResponse{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		switch st {
		case "pending":
			out.Pending = n
		case "building":
			out.Building = n
		case "ready":
			out.Ready = n
		case "error":
			out.Error = n
		case "drop_pending":
			out.DropPending = n
		}
	}
	return out, rows.Err()
}
