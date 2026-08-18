package calc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

func linkTbl(ctx context.Context) string { return postgres.TablesFromContext(ctx).QLinkRef() }

type LinkEdge struct {
	FromTableName string
	FromRecordID  string
	FromFieldID   string
	ToTableName   string
	ToRecordID    string
}

func ListToIDs(ctx context.Context, q queryRower, tenantID, fromRecordID, fromFieldID string) ([]string, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT to_record_id FROM %s
		WHERE tenant_id = $1 AND from_record_id = $2 AND from_field_id = $3
		ORDER BY id`, linkTbl(ctx)), tenantID, fromRecordID, fromFieldID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

type queryRower interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func ListOutgoing(ctx context.Context, pool *pgxpool.Pool, tenantID, fromRecordID string) ([]LinkEdge, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT from_table_name, from_record_id, from_field_id, to_table_name, to_record_id
		FROM %s WHERE tenant_id = $1 AND from_record_id = $2`, linkTbl(ctx)), tenantID, fromRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.FromTableName, &e.FromRecordID, &e.FromFieldID, &e.ToTableName, &e.ToRecordID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func ListIncoming(ctx context.Context, pool *pgxpool.Pool, tenantID, toRecordID string) ([]LinkEdge, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT from_table_name, from_record_id, from_field_id, to_table_name, to_record_id
		FROM %s WHERE tenant_id = $1 AND to_record_id = $2`, linkTbl(ctx)), tenantID, toRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.FromTableName, &e.FromRecordID, &e.FromFieldID, &e.ToTableName, &e.ToRecordID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func ReplaceLinks(ctx context.Context, tx pgx.Tx, tenantID, fromTableName, fromRecordID, fromFieldID, toTableName string, toIDs []string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE tenant_id = $1 AND from_record_id = $2 AND from_field_id = $3`, linkTbl(ctx)),
		tenantID, fromRecordID, fromFieldID); err != nil {
		return err
	}
	for _, toID := range toIDs {
		toID = trimID(toID)
		if toID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (tenant_id, from_table_name, from_record_id, from_field_id, to_table_name, to_record_id)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (from_record_id, from_field_id, to_record_id) DO NOTHING`, linkTbl(ctx)),
			tenantID, fromTableName, fromRecordID, fromFieldID, toTableName, toID); err != nil {
			return err
		}
	}
	return nil
}

func InsertInverse(ctx context.Context, tx pgx.Tx, tenantID, fromTableName, fromRecordID, inverseFieldID, toTableName, toRecordID string) error {
	if inverseFieldID == "" || toRecordID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (tenant_id, from_table_name, from_record_id, from_field_id, to_table_name, to_record_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (from_record_id, from_field_id, to_record_id) DO NOTHING`, linkTbl(ctx)),
		tenantID, toTableName, toRecordID, inverseFieldID, fromTableName, fromRecordID)
	return err
}

// DeleteEdge removes one directed link_ref edge (from_record_id, from_field_id → to_record_id).
func DeleteEdge(ctx context.Context, tx pgx.Tx, tenantID, fromRecordID, fromFieldID, toRecordID string) error {
	if fromRecordID == "" || fromFieldID == "" || toRecordID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE tenant_id = $1 AND from_record_id = $2 AND from_field_id = $3 AND to_record_id = $4`, linkTbl(ctx)),
		tenantID, fromRecordID, fromFieldID, toRecordID)
	return err
}

func DeleteInverseForField(ctx context.Context, tx pgx.Tx, tenantID, inverseFieldID string, toIDs []string, fromRecordID string) error {
	if inverseFieldID == "" || len(toIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE tenant_id = $1 AND from_field_id = $2 AND from_record_id = ANY($3) AND to_record_id = $4`, linkTbl(ctx)),
		tenantID, inverseFieldID, toIDs, fromRecordID)
	return err
}

func LinksByRecords(ctx context.Context, pool *pgxpool.Pool, tenantID string, fromRecordIDs []string) (map[string]map[string][]string, error) {
	out := map[string]map[string][]string{}
	if len(fromRecordIDs) == 0 {
		return out, nil
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT from_record_id, from_field_id, to_record_id
		FROM %s WHERE tenant_id = $1 AND from_record_id = ANY($2)
		ORDER BY id`, linkTbl(ctx)), tenantID, fromRecordIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var recID, fieldID, toID string
		if err := rows.Scan(&recID, &fieldID, &toID); err != nil {
			return nil, err
		}
		if out[recID] == nil {
			out[recID] = map[string][]string{}
		}
		out[recID][fieldID] = append(out[recID][fieldID], toID)
	}
	return out, rows.Err()
}

func trimID(s string) string {
	if s == "<nil>" {
		return ""
	}
	return s
}
