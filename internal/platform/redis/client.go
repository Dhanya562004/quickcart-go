package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrRedisUnavailable indicates that the Redis cache instance cannot be reached.
	ErrRedisUnavailable = errors.New("redis service is unavailable")
)

// Client wraps *redis.Client to provide structured logging, connection monitoring,
// and safe fallbacks when Redis is offline.
type Client struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// NewClient creates a new Redis client instance from a standard Redis URL.
// If Redis is unreachable at initialization, it logs a warning instead of panicking,
// allowing downstream components to gracefully fall back to PostgreSQL.
func NewClient(ctx context.Context, redisURL string, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL %q: %w", redisURL, err)
	}

	// Production connection pool tuning
	opt.PoolSize = 50
	opt.MinIdleConns = 10
	opt.ConnMaxIdleTime = 5 * time.Minute
	opt.DialTimeout = 2 * time.Second
	opt.ReadTimeout = 1 * time.Second
	opt.WriteTimeout = 1 * time.Second

	rdb := redis.NewClient(opt)

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		logger.Warn("redis server unreachable at boot; operating in PostgreSQL fallback mode",
			"url", opt.Addr,
			"error", err,
		)
	} else {
		logger.Info("connected to redis server", "addr", opt.Addr)
	}

	return &Client{
		rdb:    rdb,
		logger: logger,
	}, nil
}

// Raw returns the underlying *redis.Client for custom commands or scripts.
func (c *Client) Raw() *redis.Client {
	if c == nil {
		return nil
	}
	return c.rdb
}

// Ping checks if Redis is currently responsive.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return ErrRedisUnavailable
	}
	return c.rdb.Ping(ctx).Err()
}

// Close gracefully closes Redis connections.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
