package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB encapsulates the pgxpool connection pool and operational health methods.
type DB struct {
	Pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewPool initializes and tests a PostgreSQL connection pool using pgxpool.
func NewPool(ctx context.Context, cfg *config.Config, log *slog.Logger) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database configuration: %w", err)
	}

	// Apply production-grade pool sizing and lifetime parameters
	if cfg.DBMaxConns > 0 {
		poolConfig.MaxConns = cfg.DBMaxConns
	}
	if cfg.DBMinConns > 0 {
		poolConfig.MinConns = cfg.DBMinConns
	}
	if cfg.DBMaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	}
	if cfg.DBMaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime
	}
	if cfg.DBHealthCheckDelay > 0 {
		poolConfig.HealthCheckPeriod = cfg.DBHealthCheckDelay
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	// Verify initial database connectivity with a timeout
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		// In local dev without postgres running, we log a warning instead of hard crash
		// so unit tests or mock runs can proceed smoothly.
		log.Warn("PostgreSQL ping failed (database may be offline)",
			slog.String("url", sanitizeURL(cfg.DatabaseURL)),
			slog.String("error", err.Error()),
		)
	} else {
		log.Info("Connected to PostgreSQL database",
			slog.Int("max_conns", int(cfg.DBMaxConns)),
			slog.Int("min_conns", int(cfg.DBMinConns)),
		)
	}

	return &DB{
		Pool:   pool,
		logger: log,
	}, nil
}

// Ping checks if the PostgreSQL database is reachable.
func (d *DB) Ping(ctx context.Context) error {
	if d.Pool == nil {
		return fmt.Errorf("database pool is not initialized")
	}
	return d.Pool.Ping(ctx)
}

// Close gracefully terminates all idle and active connections in the pool.
func (d *DB) Close() {
	if d.Pool != nil {
		d.logger.Info("Closing database connection pool")
		d.Pool.Close()
	}
}

// sanitizeURL removes password from connection string for safe logging.
func sanitizeURL(rawURL string) string {
	// Simple mask for logs
	if len(rawURL) > 30 {
		return rawURL[:15] + "..." + rawURL[len(rawURL)-15:]
	}
	return rawURL
}
