package database

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	URL      string
	MaxConns int32
}

func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	applyLimits(pc, cfg.MaxConns)
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("db: pool: %w", err)
	}
	if err := ping(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

func applyLimits(pc *pgxpool.Config, maxConns int32) {
	pc.MaxConns = normalizeMax(maxConns)
	pc.MinConns = 1
	pc.MaxConnLifetime = time.Hour
	pc.MaxConnIdleTime = 30 * time.Minute
	pc.HealthCheckPeriod = time.Minute
}

func normalizeMax(maxConns int32) int32 {
	if maxConns <= 0 {
		maxConns = int32(runtime.NumCPU() * 2)
	}
	if maxConns < 2 {
		return 2
	}
	return maxConns
}

func ping(ctx context.Context, pool *pgxpool.Pool) error {
	pingCtx, cancel := WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}
	return nil
}
