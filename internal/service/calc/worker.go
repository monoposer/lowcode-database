package calc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/infra/postgres"
	"github.com/monoposer/lowcode-database/internal/logger"
)

// WorkerConfig configures in-process calc_queue polling (cmd/server).
type WorkerConfig struct {
	Tenants       *postgres.TenantManager
	Batch         int
	Poll          time.Duration
	Log           *logger.Logger
	PerTenant     int
	AlertQueueLen int
	Telemetry     interface {
		RecordHistogram(name string, value float64, labels map[string]string)
		IncCounter(name string, labels map[string]string)
	}
}

// Worker polls every active shard's calc_queue and writes record.data caches.
type Worker struct {
	cfg WorkerConfig
}

func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.Batch <= 0 {
		cfg.Batch = 16
	}
	if cfg.Poll <= 0 {
		cfg.Poll = 500 * time.Millisecond
	}
	return &Worker{cfg: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.cfg.Tenants == nil {
		return
	}
	t := time.NewTicker(w.cfg.Poll)
	defer t.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	shards, err := w.cfg.Tenants.ListActiveDataStores(ctx)
	if err != nil || len(shards) == 0 {
		return
	}
	meta := w.cfg.Tenants.MetaPool()
	for _, sh := range shards {
		pool, err := w.cfg.Tenants.PoolForTenant(ctx, sh.TenantID)
		if err != nil {
			w.log("tenant pool", "tenant_id", sh.TenantID, "err", err)
			continue
		}
		sctx := postgres.WithDataTables(ctx, sh.Tables)
		_ = postgres.EnsureDataTables(sctx, pool, sh.Tables)
		if err := w.Drain(sctx, meta, pool); err != nil {
			w.log("drain", "tenant_id", sh.TenantID, "err", err)
		}
	}
}

// Drain claims and processes one batch on a single data pool.
func (w *Worker) Drain(ctx context.Context, meta, data *pgxpool.Pool) error {
	tasks, err := ClaimFair(ctx, data, w.cfg.Batch, w.cfg.PerTenant)
	if err != nil {
		return err
	}
	eng := &Engine{Meta: meta, Data: data}
	for _, t := range tasks {
		start := time.Now()
		err := eng.ProcessOrRetry(ctx, t)
		if w.cfg.Telemetry != nil {
			w.cfg.Telemetry.RecordHistogram("calc.task.duration_ms", float64(time.Since(start).Milliseconds()), map[string]string{
				"tenant_id": t.TenantID, "table_id": t.TableID, "api_operation": "calc.process",
			})
			if err != nil {
				w.cfg.Telemetry.IncCounter("calc.task.failures", map[string]string{"tenant_id": t.TenantID})
			}
		}
		if err != nil {
			w.log("task", "id", t.ID, "record", t.RecordID, "err", err)
		}
	}
	if w.cfg.AlertQueueLen > 0 {
		if snap, err := Snapshot(ctx, data, w.cfg.AlertQueueLen); err == nil && snap.Alert {
			w.log("queue alert", "pending", snap.Pending, "failed", snap.Failed, "dead_letters", snap.DeadLetters)
		}
	}
	return nil
}

func (w *Worker) log(msg string, kv ...any) {
	if w.cfg.Log != nil {
		w.cfg.Log.Info("calc worker "+msg, kv...)
	}
}
