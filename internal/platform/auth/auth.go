package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/solat/lowcode-database/internal/config"
	"net/http"
	"strings"
	"sync"
	"time"
)

const apiKeyHeader = "X-Api-Key"

// Validator checks API keys and enforces per-key rate limits.
type Validator struct {
	pool       *pgxpool.Pool
	required   bool
	defaultRPS float64
	limiters   sync.Map // keyHash -> *tokenBucket
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	refill   float64
	last     time.Time
}

func NewValidator(pool *pgxpool.Pool, cfg *config.Config) *Validator {
	rps := cfg.RateLimitRPS
	if rps <= 0 {
		rps = 100
	}
	return &Validator{
		pool:       pool,
		required:   cfg.APIKeyRequired,
		defaultRPS: float64(rps),
	}
}

type keyRecord struct {
	TenantID     string
	KeyHash      string
	Enabled      bool
	RateLimitRPS int
}

func extractAPIKey(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get(apiKeyHeader)); k != "" {
		return k
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func isPublicPath(path string) bool {
	switch {
	case path == "/" || path == "/swagger/" || strings.HasPrefix(path, "/swagger/"):
		return true
	case path == "/metrics":
		return true
	case path == "/v1/admin/events/schemas" || path == "/v1/admin/events/envelope-schema":
		return true
	default:
		return false
	}
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateKey creates a random API key string and its hash/prefix.
func GenerateKey() (plain, hash, prefix string, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", "", err
	}
	plain = "lc_" + hex.EncodeToString(b[:])
	hash = hashKey(plain)
	prefix = plain[:10]
	return plain, hash, prefix, nil
}
