package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/monoposer/lowcode-database/internal/tenant"
)

// Limiter enforces a global token bucket plus a per-tenant bucket.
type Limiter struct {
	global    *bucket
	tenant    *sync.Map // tenantID -> *bucket
	tenantRPS float64
}

type bucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	refill   float64
	last     time.Time
}

func newBucket(rps float64) *bucket {
	if rps <= 0 {
		return nil
	}
	return &bucket{tokens: rps, capacity: rps, refill: rps, last: time.Now()}
}

func (b *bucket) allow() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.refill
	b.last = now
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// New returns a limiter. Zero RPS disables that layer.
func New(globalRPS, tenantRPS int) *Limiter {
	l := &Limiter{tenant: &sync.Map{}, tenantRPS: float64(tenantRPS)}
	if globalRPS > 0 {
		l.global = newBucket(float64(globalRPS))
	}
	return l
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.global != nil && !l.global.allow() {
			http.Error(w, "global rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		if l.tenantRPS > 0 {
			tid := tenant.ResolveTenantID(r.Context())
			if tid == "" {
				tid = "_none"
			}
			val, _ := l.tenant.LoadOrStore(tid, newBucket(l.tenantRPS))
			if !val.(*bucket).allow() {
				http.Error(w, "tenant rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
