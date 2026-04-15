package transport

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfig carries the minimum configuration needed to bootstrap a shared pool.
type PostgresConfig struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	HealthCheckPeriod time.Duration
}

// OpenPostgres creates and validates a pgx connection pool.
func OpenPostgres(ctx context.Context, cfg PostgresConfig) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("postgres url is required")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	if cfg.HealthCheckPeriod > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		pool.Close()
	}()

	return pool, nil
}
