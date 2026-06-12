package authn

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/monoposer/lowcode-database/internal/tenant"
)

func (v *Validator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v == nil || v.pool == nil {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		rawKey := extractAPIKey(r)
		if rawKey == "" {
			if v.required {
				http.Error(w, "api key required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		keyRec, err := v.lookupKey(r.Context(), rawKey)
		if err != nil {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}
		if !keyRec.Enabled {
			http.Error(w, "api key disabled", http.StatusUnauthorized)
			return
		}
		rps := v.defaultRPS
		if keyRec.RateLimitRPS > 0 {
			rps = float64(keyRec.RateLimitRPS)
		}
		if !v.allow(keyRec.KeyHash, rps) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		tid := strings.TrimSpace(tenant.FromContext(r.Context()))
		if tid == "" {
			tid = keyRec.TenantID
		}
		ctx := tenant.WithTenantID(r.Context(), keyRec.TenantID)
		if tid != "" && tid != keyRec.TenantID {
			http.Error(w, "tenant mismatch for api key", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (v *Validator) lookupKey(ctx context.Context, raw string) (*keyRecord, error) {
	hash := hashKey(raw)
	var rec keyRecord
	err := v.pool.QueryRow(ctx, `
		SELECT tenant_id, key_hash, enabled, rate_limit_rps
		FROM lc_api_keys WHERE key_hash = $1
	`, hash).Scan(&rec.TenantID, &rec.KeyHash, &rec.Enabled, &rec.RateLimitRPS)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (v *Validator) allow(keyHash string, rps float64) bool {
	if rps <= 0 {
		return true
	}
	val, _ := v.limiters.LoadOrStore(keyHash, &tokenBucket{
		tokens:   rps,
		capacity: rps,
		refill:   rps,
		last:     time.Now(),
	})
	b := val.(*tokenBucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.refill
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
