package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/monoposer/lowcode-database/internal/config"
	"time"
)

// NewPool creates a pgx connection pool using META_DATABASE_URL from config.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.MetaDatabaseURL == "" {
		return nil, fmt.Errorf("META_DATABASE_URL is not set")
	}
	return NewPoolFromDSN(ctx, cfg.MetaDatabaseURL, PoolSettingsFromConfig(cfg), 0)
}

// NewPoolFromDSN creates a pgx connection pool from a full DSN.
func NewPoolFromDSN(ctx context.Context, dsn string, settings PoolSettings, maxConnsOverride int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	cfg.MaxConns = settings.effectiveMaxConns(maxConnsOverride)
	cfg.MinConns = settings.MinConns
	if cfg.MinConns <= 0 {
		cfg.MinConns = 1
	}
	life := settings.MaxConnLifetime
	if life <= 0 {
		life = time.Hour
	}
	cfg.MaxConnLifetime = life

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return pool, nil
}

// PoolSettings configures pgxpool defaults for meta and tenant data pools.
type PoolSettings struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxTenantPools  int
}

func PoolSettingsFromConfig(cfg *config.Config) PoolSettings {
	if cfg == nil {
		return PoolSettings{MaxConns: 10, MinConns: 1, MaxConnLifetime: time.Hour}
	}
	max := int32(cfg.PGMaxConns)
	if max <= 0 {
		max = 10
	}
	min := int32(cfg.PGMinConns)
	if min <= 0 {
		min = 1
	}
	lifeMin := cfg.PGMaxConnLifetimeMin
	if lifeMin <= 0 {
		lifeMin = 60
	}
	return PoolSettings{
		MaxConns:        max,
		MinConns:        min,
		MaxConnLifetime: time.Duration(lifeMin) * time.Minute,
		MaxTenantPools:  cfg.MaxTenantDataPools,
	}
}

func (s PoolSettings) effectiveMaxConns(override int) int32 {
	if override > 0 {
		return int32(override)
	}
	return s.MaxConns
}
