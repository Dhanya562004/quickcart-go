package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_PostgresAndRedisContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. PostgreSQL 16 Container with Automatic Migrations
	pgConnStr, pgCleanup := SetupPostgresContainer(ctx, t)
	defer pgCleanup()

	if pgConnStr != "" {
		pool, err := pgxpool.New(ctx, pgConnStr)
		require.NoError(t, err)
		defer pool.Close()

		// Verify migrations were applied: check users and refresh_tokens tables exist
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'refresh_tokens')").Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "refresh_tokens table must exist after migration")

		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'orders')").Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "orders table must exist after migration")
	}

	// 2. Redis 7 Container
	redisURL, redisCleanup := SetupRedisContainer(ctx, t)
	defer redisCleanup()

	if redisURL != "" {
		opts, err := goredis.ParseURL(redisURL)
		require.NoError(t, err)

		rdb := goredis.NewClient(opts)
		defer func() {
			_ = rdb.Close()
		}()

		pong, err := rdb.Ping(ctx).Result()
		require.NoError(t, err)
		assert.Equal(t, "PONG", pong)

		// Test key set and get
		err = rdb.Set(ctx, "test_key", "quickcart_val", time.Minute).Err()
		require.NoError(t, err)
		val, err := rdb.Get(ctx, "test_key").Result()
		require.NoError(t, err)
		assert.Equal(t, "quickcart_val", val)
	}
}
