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
	// MetaDatabaseURL is the Postgres URL for **metadata** (all lc_* tables + lc_tenants), shared by every tenant.
	MetaDatabaseURL string

	// DataAdminDatabaseURL optional superuser DSN (e.g. .../postgres) used with create_database when provisioning tenants.
	DataAdminDatabaseURL string
	// DataDSNTemplate optional printf template for tenant data DSN when API omits data_dsn, e.g. postgresql://u:p@host:5432/%s
	DataDSNTemplate string

	// DefaultTenantID + DefaultTenantDataDSN bootstrap one lc_tenants row on startup (e.g. single-tenant stack).
	DefaultTenantID        string
	DefaultTenantDataDSN   string

	HTTPAddr string
	MaxRow   int

	// Redis (optional): metadata cache + metrics backend
	RedisURL       string
	CacheEnabled   bool
	CacheTTLSeconds int

	// MetricsBackend: noop | redis | prometheus
	MetricsBackend    string
	MetricsWindowSize int

	// SlowQueryThresholdMS triggers warn logs when SQL exceeds this duration.
	SlowQueryThresholdMS int
	LogLevel             string
}

// Load reads configuration from environment variables, optionally populating
// them from a local ".env" file if present.
func Load() (*Config, error) {
	loadDotEnvIfPresent()

	cfg := &Config{
		MetaDatabaseURL:        firstNonEmpty(os.Getenv("META_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		DataAdminDatabaseURL:   os.Getenv("DATA_ADMIN_DATABASE_URL"),
		DataDSNTemplate:        os.Getenv("DATA_DSN_TEMPLATE"),
		DefaultTenantID:        getenvDefault("DEFAULT_TENANT_ID", "default"),
		DefaultTenantDataDSN:   firstNonEmpty(os.Getenv("DEFAULT_TENANT_DATA_DSN"), os.Getenv("SINGLE_DATABASE_URL")),
		HTTPAddr:               getenvDefault("HTTP_ADDR", ":8080"),
		MaxRow:                 getenvInt("MAX_ROW", 100),
		RedisURL:               os.Getenv("REDIS_URL"),
		CacheEnabled:           getenvBool("CACHE_ENABLED", os.Getenv("REDIS_URL") != ""),
		CacheTTLSeconds:        getenvInt("CACHE_TTL_SECONDS", 300),
		MetricsBackend:         getenvDefault("METRICS_BACKEND", "noop"),
		MetricsWindowSize:      getenvInt("METRICS_WINDOW_SIZE", 100),
		SlowQueryThresholdMS:   getenvInt("SLOW_QUERY_THRESHOLD_MS", 500),
		LogLevel:               getenvDefault("LOG_LEVEL", "info"),
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
