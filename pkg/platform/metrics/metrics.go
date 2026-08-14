package metrics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// Statement is one row from pg_stat_statements (current database).
type Statement struct {
	QueryID        string  `json:"queryId,omitempty"`
	Query          string  `json:"query,omitempty"`
	Calls          int64   `json:"calls"`
	TotalExecTime  float64 `json:"totalExecTimeMs"`
	MeanExecTime   float64 `json:"meanExecTimeMs"`
	MinExecTime    float64 `json:"minExecTimeMs"`
	MaxExecTime    float64 `json:"maxExecTimeMs"`
	Rows           int64   `json:"rows"`
	SharedBlksHit  int64   `json:"sharedBlksHit"`
	SharedBlksRead int64   `json:"sharedBlksRead"`
}

// ClampLimit bounds list size for the statements API.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// List reads pg_stat_statements for the connected database, ordered by total time.
func List(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Statement, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}
	limit = ClampLimit(limit)
	rows, err := pool.Query(ctx, `
		SELECT
			queryid::text,
			query,
			calls,
			total_exec_time,
			mean_exec_time,
			min_exec_time,
			max_exec_time,
			rows,
			shared_blks_hit,
			shared_blks_read
		FROM pg_stat_statements
		WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		ORDER BY total_exec_time DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_statements: %w (enable PG_STAT_STATEMENTS, shared_preload_libraries=pg_stat_statements, and CREATE EXTENSION)", err)
	}
	defer rows.Close()

	out := []Statement{}
	for rows.Next() {
		var (
			queryID *string
			s       Statement
		)
		if err := rows.Scan(
			&queryID,
			&s.Query,
			&s.Calls,
			&s.TotalExecTime,
			&s.MeanExecTime,
			&s.MinExecTime,
			&s.MaxExecTime,
			&s.Rows,
			&s.SharedBlksHit,
			&s.SharedBlksRead,
		); err != nil {
			return nil, fmt.Errorf("scan pg_stat_statements: %w", err)
		}
		if queryID != nil {
			s.QueryID = *queryID
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
