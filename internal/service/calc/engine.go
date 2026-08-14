package calc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// Engine processes calc_queue tasks against a tenant shard + global meta.
type Engine struct {
	Meta *pgxpool.Pool
	Data *pgxpool.Pool
}

func (e *Engine) Process(ctx context.Context, t Task) error {
	if e == nil || e.Meta == nil || e.Data == nil {
		return fmt.Errorf("calc engine: pools required")
	}
	tm, err := loadTableMeta(ctx, e.Meta, t.TenantID, t.TableName)
	if err != nil {
		return err
	}
	vtMap, err := loadVTMap(ctx, e.Meta, t.TenantID)
	if err != nil {
		return err
	}
	fields := enrichLookupRollupTargets(tm.Fields)
	rec, err := ReadRecord(ctx, e.Data, t.TenantID, tm.VTID, t.RecordID)
	if err != nil {
		return err
	}
	patched, err := ComputeFields(ctx, e.Data, rec, fields, nil, vtMap, t.TargetFieldIDs)
	if err != nil {
		return err
	}
	if len(patched) == 0 {
		return MarkCompleted(ctx, e.Data, t.ID)
	}
	ok, err := PatchCalcFields(ctx, e.Data, rec, patched)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w", ErrVersionConflict)
	}
	return MarkCompleted(ctx, e.Data, t.ID)
}

func (e *Engine) ProcessOrRetry(ctx context.Context, t Task) error {
	err := e.Process(ctx, t)
	if err == nil {
		return nil
	}
	if t.RetryCount+1 >= t.MaxRetry && t.MaxRetry > 0 {
		writeErrorCaches(ctx, e, t)
	}
	return MarkRetry(ctx, e.Data, t, err)
}

func writeErrorCaches(ctx context.Context, e *Engine, t Task) {
	tm, err := loadTableMeta(ctx, e.Meta, t.TenantID, t.TableName)
	if err != nil {
		return
	}
	rec, err := ReadRecord(ctx, e.Data, t.TenantID, tm.VTID, t.RecordID)
	if err != nil {
		return
	}
	patched := map[string]any{}
	for _, f := range tm.Fields {
		if !IsCalcType(f.TypeID) {
			continue
		}
		if len(t.TargetFieldIDs) > 0 {
			match := false
			for _, id := range t.TargetFieldIDs {
				if id == f.Name || id == f.ID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		patched[f.Name] = WrapCache(UnwrapCache(rec.Data[f.Name]), CacheError)
	}
	_, _ = PatchCalcFields(ctx, e.Data, rec, patched)
}

// EnqueueAfterUserEdit fans DAG from changed fields + optional link mutation. Does not bump record.version.
func EnqueueAfterUserEdit(ctx context.Context, meta, data *pgxpool.Pool, tenantID, tableName, recordID string, changed []string, linkChanged bool) error {
	tm, err := loadTableMeta(ctx, meta, tenantID, tableName)
	if err != nil {
		return err
	}
	changedSet := map[string]bool{}
	for _, c := range changed {
		changedSet[c] = true
	}
	needSelf := linkChanged
	for _, f := range tm.Fields {
		if f.TypeID != "formula" {
			continue
		}
		expr := shared.FormulaExpression(f.Config)
		for _, ref := range formulacompile.Refs(expr) {
			if changedSet[ref] {
				needSelf = true
				break
			}
		}
		for _, d := range fieldDeps(f) {
			if changedSet[d] {
				needSelf = true
			}
		}
	}
	for _, f := range tm.Fields {
		if (f.TypeID == "lookup" || f.TypeID == "rollup") && linkChanged {
			needSelf = true
		}
	}
	if needSelf {
		if err := Enqueue(ctx, data, tenantID, tm.Name, recordID, nil); err != nil {
			return err
		}
	}
	incoming, err := ListIncoming(ctx, data, tenantID, recordID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, e := range incoming {
		if seen[e.FromRecordID] {
			continue
		}
		seen[e.FromRecordID] = true
		logical := e.FromTableName
		if err := Enqueue(ctx, data, tenantID, logical, e.FromRecordID, nil); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) LiveCompute(ctx context.Context, tenantID, tableName, recordID string) (map[string]any, int64, error) {
	tm, err := loadTableMeta(ctx, e.Meta, tenantID, tableName)
	if err != nil {
		return nil, 0, err
	}
	vtMap, err := loadVTMap(ctx, e.Meta, tenantID)
	if err != nil {
		return nil, 0, err
	}
	rec, err := ReadRecord(ctx, e.Data, tenantID, tm.VTID, recordID)
	if err != nil {
		return nil, 0, err
	}
	fields := enrichLookupRollupTargets(tm.Fields)
	live, err := ComputeFields(ctx, e.Data, rec, fields, nil, vtMap, nil)
	if err != nil {
		return nil, rec.Version, err
	}
	merged := map[string]any{}
	for k, v := range rec.Data {
		merged[k] = UnwrapCache(v)
	}
	for k, v := range live {
		merged[k] = UnwrapCache(v)
	}
	return merged, rec.Version, nil
}

func CacheMismatch(cached, live map[string]any, fields []Field) bool {
	for _, f := range fields {
		if !IsCalcType(f.TypeID) {
			continue
		}
		cv := fmt.Sprint(UnwrapCache(cached[f.Name]))
		lv := fmt.Sprint(UnwrapCache(live[f.Name]))
		if cv != lv {
			return true
		}
	}
	return false
}
