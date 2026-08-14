package platform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/platform/cache"
)

type RuntimeResponse struct {
	DataPools     any                 `json:"dataPools"`
	PoolCount     int                 `json:"poolCount"`
	CreateWaiters int32               `json:"createWaiters"`
	Calc          *calc.QueueSnapshot `json:"calc,omitempty"`
}

func (s *Platform) Runtime(ctx context.Context) (*RuntimeResponse, error) {
	out := &RuntimeResponse{}
	if s.B.Tenants != nil {
		stats := s.B.Tenants.DataPoolStats()
		out.DataPools = stats
		out.PoolCount = s.B.Tenants.ActiveDataPoolCount()
		out.CreateWaiters = s.B.Tenants.CreateWaiters()
		if pool, err := s.B.Tenants.DataPool(ctx); err == nil {
			ctx, _, _ = s.B.Tenants.AttachDataTables(ctx)
			snap, err := calc.Snapshot(ctx, pool, s.B.CalcAlertQueueLen)
			if err == nil {
				out.Calc = &snap
			}
		}
	}
	return out, nil
}

type CacheInspectResponse struct {
	Key        string `json:"key"`
	CacheHit   bool   `json:"cacheHit"`
	Consistent bool   `json:"consistent"`
	Detail     string `json:"detail,omitempty"`
}

func (s *Platform) InspectCache(ctx context.Context, tableName string) (*CacheInspectResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if tableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	key := shared.CacheKeyColumns(tid, tableName)
	out := &CacheInspectResponse{Key: key}
	raw, ok := cacheRaw(s.B.Cache, ctx, key)
	out.CacheHit = ok
	if !ok {
		out.Detail = "cache miss (ok if cold)"
		out.Consistent = true
		return out, nil
	}
	var cached shared.CachedColumnMetaBundle
	if err := json.Unmarshal(raw, &cached); err != nil {
		out.Detail = "cached payload is not column bundle"
		return out, nil
	}
	var dbCount int
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT COUNT(*)::int FROM lc_columns
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3`, tid, baseID, tableName).Scan(&dbCount); err != nil {
		return nil, err
	}
	if dbCount != len(cached.Cols) {
		out.Consistent = false
		out.Detail = fmt.Sprintf("column count cache=%d db=%d", len(cached.Cols), dbCount)
		return out, nil
	}
	out.Consistent = true
	out.Detail = "ok"
	return out, nil
}

func cacheRaw(c cache.MetaCache, ctx context.Context, key string) ([]byte, bool) {
	if g, ok := c.(cache.RawGetter); ok {
		b, found, err := g.GetRaw(ctx, key)
		if err != nil || !found {
			return nil, false
		}
		return b, true
	}
	var dest json.RawMessage
	ok, err := c.Get(ctx, key, &dest)
	if err != nil || !ok {
		return nil, false
	}
	return dest, true
}
