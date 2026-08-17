package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/pkg/logger"
)

type sqlQueryTracer struct {
	log *logger.Logger
}

type sqlTraceData struct {
	start time.Time
	sql   string
	args  []any
}

type sqlTraceCtxKey struct{}

func newSQLQueryTracer(log *logger.Logger) pgx.QueryTracer {
	if log == nil {
		return nil
	}
	return &sqlQueryTracer{log: log}
}

func (t *sqlQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, sqlTraceCtxKey{}, sqlTraceData{
		start: time.Now(),
		sql:   data.SQL,
		args:  data.Args,
	})
}

func (t *sqlQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if !t.log.Enabled(ctx, slog.LevelDebug) && data.Err == nil {
		return
	}
	td, _ := ctx.Value(sqlTraceCtxKey{}).(sqlTraceData)
	attrs := []any{
		"sql", td.sql,
		"args", logger.FormatSQLArgs(td.args),
		"duration_ms", time.Since(td.start).Milliseconds(),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err.Error())
		t.log.Warn("sql", attrs...)
		return
	}
	t.log.Debug("sql", attrs...)
}
