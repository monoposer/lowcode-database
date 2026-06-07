package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds service configuration from environment variables.
type Config struct {
	// MetaDatabaseURL is the Postgres URL for **metadata** (tenants, lc_bases, lc_*), shared by every tenant.
	MetaDatabaseURL string

	// DataAdminDatabaseURL optional superuser DSN (e.g. .../postgres) used with create_database when provisioning tenants.
	DataAdminDatabaseURL string
	// DataDSNTemplate optional printf template for tenant data DSN when API omits data_dsn, e.g. postgresql://u:p@host:5432/%s
	DataDSNTemplate string

	// DefaultTenantID + DefaultTenantDataDSN bootstrap one tenant (+ default base) on startup.
	DefaultTenantID      string
	DefaultTenantDataDSN string

	HTTPAddr string
	MaxRow   int

	// Redis (optional): metadata cache
	RedisURL        string
	CacheEnabled    bool
	CacheTTLSeconds int

	// SlowQueryThresholdMS triggers warn logs when SQL exceeds this duration.
	SlowQueryThresholdMS int
	LogLevel             string
	// LogSQL logs row-query SQL and bind args at info level (for local debugging).
	LogSQL bool

	// PGStatStatements enables pg_stat_statements listing via GET /v1/admin/pg-stat-statements.
	PGStatStatements bool

	// APIKeyRequired rejects /v1/* without a valid X-Api-Key when true.
	APIKeyRequired bool
	// RateLimitRPS default per API key (0 = 100).
	RateLimitRPS int

	// Postgres pool defaults (meta + tenant data pools).
	PGMaxConns           int
	PGMinConns           int
	PGMaxConnLifetimeMin int
	MaxTenantDataPools   int
	DefaultTenantPoolMax int
	// PoolHotIdleSeconds: pools used within this window are not LRU-evicted (hot keep-alive).
	PoolHotIdleSeconds int
	// PoolCreateWaitMS: wait this long for an in-flight pool create after eviction (anti-stampede).
	PoolCreateWaitMS int

	// HTTP overload protection (in addition to per-API-key limits).
	RateLimitGlobalRPS int
	RateLimitTenantRPS int

	// Data API guards.
	MaxScanRows   int
	MaxBulkItems  int
	MaxExportRows int

	// Calc worker reliability.
	CalcMaxRetry          int
	CalcTenantConcurrency int
	CalcAlertQueueLen     int

	// Admin DDL: drop table/column require X-Confirm-Dangerous matching the resource id.
	DDLConfirmRequired bool

	// Index backfill timeout (seconds) for CONCURRENTLY DDL.
	IndexBackfillTimeoutSec int

	// TenantIsolationMode is always virtual_records (TENANT_ISOLATION_MODE env is ignored).
	TenantIsolationMode TenantIsolationMode
	// TenantDataSchemaPrefix is unused in virtual_records (kept for shared.Base compat).
	TenantDataSchemaPrefix string
	// VRDefaultShardDSN is an alias for the default tenant data DSN (defaults to DefaultTenantDataDSN).
	VRDefaultShardDSN string

	// Calc loop (formula / lookup / rollup cache via calc_queue) always runs in cmd/server.
	CalcWorkerBatch  int
	CalcWorkerPollMS int

	// EventBus: "memory" (default) or "redis". Redis requires REDIS_URL.
	EventBus       string
	EventStreamKey string
}

// Load reads configuration from environment variables, optionally populating
// them from a local ".env" file if present.
func Load() (*Config, error) {
	loadDotEnvIfPresent()

	cfg := &Config{
		MetaDatabaseURL:         firstNonEmpty(os.Getenv("META_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		DataAdminDatabaseURL:    os.Getenv("DATA_ADMIN_DATABASE_URL"),
		DataDSNTemplate:         os.Getenv("DATA_DSN_TEMPLATE"),
		DefaultTenantID:         getenvDefault("DEFAULT_TENANT_ID", "default"),
		DefaultTenantDataDSN:    firstNonEmpty(os.Getenv("DEFAULT_TENANT_DATA_DSN"), os.Getenv("SINGLE_DATABASE_URL")),
		HTTPAddr:                getenvDefault("HTTP_ADDR", ":8080"),
		MaxRow:                  getenvInt("MAX_ROW", 1000),
		RedisURL:                os.Getenv("REDIS_URL"),
		CacheEnabled:            getenvBool("CACHE_ENABLED", os.Getenv("REDIS_URL") != ""),
		CacheTTLSeconds:         getenvInt("CACHE_TTL_SECONDS", 300),
		SlowQueryThresholdMS:    getenvInt("SLOW_QUERY_THRESHOLD_MS", 500),
		LogLevel:                getenvDefault("LOG_LEVEL", "info"),
		LogSQL:                  getenvBool("LOG_SQL", false),
		PGStatStatements:        getenvBool("PG_STAT_STATEMENTS", false),
		APIKeyRequired:          getenvBool("API_KEY_REQUIRED", false),
		RateLimitRPS:            getenvInt("RATE_LIMIT_RPS", 100),
		PGMaxConns:              getenvInt("PG_MAX_CONNS", 10),
		PGMinConns:              getenvInt("PG_MIN_CONNS", 1),
		PGMaxConnLifetimeMin:    getenvInt("PG_MAX_CONN_LIFETIME_MIN", 60),
		MaxTenantDataPools:      getenvInt("MAX_TENANT_DATA_POOLS", 50),
		DefaultTenantPoolMax:    getenvInt("DEFAULT_TENANT_POOL_MAX_CONNS", 0),
		PoolHotIdleSeconds:      getenvInt("POOL_HOT_IDLE_SECONDS", 300),
		PoolCreateWaitMS:        getenvInt("POOL_CREATE_WAIT_MS", 2000),
		RateLimitGlobalRPS:      getenvInt("RATE_LIMIT_GLOBAL_RPS", 0),
		RateLimitTenantRPS:      getenvInt("RATE_LIMIT_TENANT_RPS", 0),
		MaxScanRows:             getenvInt("MAX_SCAN_ROWS", 0),
		MaxBulkItems:            getenvInt("MAX_BULK_ITEMS", 500),
		MaxExportRows:           getenvInt("MAX_EXPORT_ROWS", 100000),
		CalcMaxRetry:            getenvInt("CALC_MAX_RETRY", 8),
		CalcTenantConcurrency:   getenvInt("CALC_TENANT_CONCURRENCY", 8),
		CalcAlertQueueLen:       getenvInt("CALC_ALERT_QUEUE_LEN", 10000),
		DDLConfirmRequired:      getenvBool("DDL_CONFIRM_REQUIRED", true),
		IndexBackfillTimeoutSec: getenvInt("INDEX_BACKFILL_TIMEOUT_SEC", 300),
		TenantIsolationMode:     TenantIsolationRLSTable,
		TenantDataSchemaPrefix:  getenvDefault("TENANT_DATA_SCHEMA_PREFIX", DefaultTenantDataSchemaPrefix),
		VRDefaultShardDSN: firstNonEmpty(
			os.Getenv("VR_DEFAULT_SHARD_DSN"),
			os.Getenv("DEFAULT_TENANT_DATA_DSN"),
			os.Getenv("SINGLE_DATABASE_URL"),
		),
		CalcWorkerBatch:  getenvInt("CALC_WORKER_BATCH", 16),
		CalcWorkerPollMS: getenvInt("CALC_WORKER_POLL_MS", 500),
		EventBus:         strings.ToLower(strings.TrimSpace(getenvDefault("EVENT_BUS", "memory"))),
		EventStreamKey:   getenvDefault("EVENT_STREAM_KEY", "lc:events"),
	}

	if cfg.MetaDatabaseURL == "" {
		return nil, fmt.Errorf("META_DATABASE_URL is required (legacy DATABASE_URL is accepted as fallback)")
	}

	return cfg, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getenvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func loadDotEnvIfPresent() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}
