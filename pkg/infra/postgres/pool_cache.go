package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type poolEntry struct {
	pool     *pgxpool.Pool
	dsn      string
	lastUsed time.Time
	hits     int64
}

// DataPoolStat is a snapshot of one cached data pool (DSN-keyed).
type DataPoolStat struct {
	Fingerprint   string `json:"fingerprint"`
	Acquired      int32  `json:"acquired"`
	Idle          int32  `json:"idle"`
	Constructing  int32  `json:"constructing"`
	MaxConns      int32  `json:"maxConns"`
	TotalConns    int32  `json:"totalConns"`
	CreateWaiters int32  `json:"createWaiters"`
	Hot           bool   `json:"hot"`
	Hits          int64  `json:"hits"`
}

func dsnPoolKey(dsn string) string {
	return strings.TrimSpace(dsn)
}

// DSNFingerprint is a non-secret label for metrics (db name + short hash).
func DSNFingerprint(dsn string) string {
	sum := sha256.Sum256([]byte(dsn))
	short := hex.EncodeToString(sum[:6])
	u, err := url.Parse(dsn)
	if err != nil || u.Path == "" {
		return short
	}
	db := strings.TrimPrefix(u.Path, "/")
	if i := strings.IndexByte(db, '?'); i >= 0 {
		db = db[:i]
	}
	if db == "" {
		return short
	}
	return db + ":" + short
}

func (m *TenantManager) touchPoolKey(key string) {
	for i, k := range m.dataPoolOrder {
		if k == key {
			m.dataPoolOrder = append(append(m.dataPoolOrder[:i], m.dataPoolOrder[i+1:]...), key)
			return
		}
	}
	m.dataPoolOrder = append(m.dataPoolOrder, key)
}

func (m *TenantManager) isHotLocked(e *poolEntry) bool {
	if e == nil {
		return false
	}
	idle := m.poolSettings.HotIdle
	if idle <= 0 {
		idle = 5 * time.Minute
	}
	return time.Since(e.lastUsed) < idle
}

func (m *TenantManager) evictPoolIfNeeded() {
	limit := m.poolSettings.MaxTenantPools
	if limit <= 0 || len(m.dataPools) < limit {
		return
	}
	var evictKey string
	for _, key := range m.dataPoolOrder {
		e := m.dataPools[key]
		if e != nil && m.isHotLocked(e) {
			continue
		}
		evictKey = key
		break
	}
	if evictKey == "" {
		return
	}
	var order []string
	for _, k := range m.dataPoolOrder {
		if k != evictKey {
			order = append(order, k)
		}
	}
	m.dataPoolOrder = order
	if e, ok := m.dataPools[evictKey]; ok {
		if e.pool != nil {
			e.pool.Close()
		}
		delete(m.dataPools, evictKey)
	}
}

func (m *TenantManager) getOrCreatePool(ctx context.Context, key, dsn string, maxConns int) (*pgxpool.Pool, error) {
	key = dsnPoolKey(key)
	if key == "" {
		key = dsnPoolKey(dsn)
	}

	m.mu.RLock()
	if e, ok := m.dataPools[key]; ok && e != nil && e.pool != nil {
		m.mu.RUnlock()
		m.mu.Lock()
		if e2, ok := m.dataPools[key]; ok && e2 != nil {
			e2.lastUsed = time.Now()
			e2.hits++
			m.touchPoolKey(key)
		}
		m.mu.Unlock()
		return e.pool, nil
	}
	m.mu.RUnlock()

	wait := m.poolSettings.CreateWait
	if wait <= 0 {
		wait = 2 * time.Second
	}
	atomic.AddInt32(&m.createWaiters, 1)
	defer atomic.AddInt32(&m.createWaiters, -1)

	ch := m.poolSF.DoChan(key, func() (any, error) {
		m.mu.Lock()
		if e, ok := m.dataPools[key]; ok && e != nil && e.pool != nil {
			e.lastUsed = time.Now()
			e.hits++
			m.touchPoolKey(key)
			p := e.pool
			m.mu.Unlock()
			return p, nil
		}
		m.evictPoolIfNeeded()
		m.mu.Unlock()

		p, err := NewPoolFromDSN(ctx, dsn, m.poolSettings, maxConns)
		if err != nil {
			return nil, err
		}

		m.mu.Lock()
		if e, ok := m.dataPools[key]; ok && e != nil && e.pool != nil {
			m.mu.Unlock()
			p.Close()
			return e.pool, nil
		}
		m.dataPools[key] = &poolEntry{pool: p, dsn: dsn, lastUsed: time.Now(), hits: 1}
		m.touchPoolKey(key)
		m.mu.Unlock()
		return p, nil
	})

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		p, _ := res.Val.(*pgxpool.Pool)
		return p, nil
	case <-timer.C:
		select {
		case res := <-ch:
			if res.Err != nil {
				return nil, res.Err
			}
			p, _ := res.Val.(*pgxpool.Pool)
			return p, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// DataPoolStats returns per-pool connection stats for metrics.
func (m *TenantManager) DataPoolStats() []DataPoolStat {
	m.mu.RLock()
	defer m.mu.RUnlock()
	waiters := atomic.LoadInt32(&m.createWaiters)
	out := make([]DataPoolStat, 0, len(m.dataPools))
	for key, e := range m.dataPools {
		if e == nil || e.pool == nil {
			continue
		}
		st := e.pool.Stat()
		out = append(out, DataPoolStat{
			Fingerprint:   DSNFingerprint(key),
			Acquired:      st.AcquiredConns(),
			Idle:          st.IdleConns(),
			Constructing:  st.ConstructingConns(),
			MaxConns:      st.MaxConns(),
			TotalConns:    st.TotalConns(),
			CreateWaiters: waiters,
			Hot:           m.isHotLocked(e),
			Hits:          e.hits,
		})
	}
	return out
}

// CreateWaiters is the number of goroutines waiting on pool create (anti-stampede queue).
func (m *TenantManager) CreateWaiters() int32 {
	return atomic.LoadInt32(&m.createWaiters)
}
