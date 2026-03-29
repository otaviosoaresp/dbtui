package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnConfig struct {
	DSN              string
	MaxConns         int32
	HealthCheckPeriod time.Duration
}

func DefaultConnConfig(dsn string) ConnConfig {
	return ConnConfig{
		DSN:              dsn,
		MaxConns:         5,
		HealthCheckPeriod: 30 * time.Second,
	}
}

func Connect(ctx context.Context, cfg ConnConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return pool, nil
}

func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return pool.Ping(ctx)
}
