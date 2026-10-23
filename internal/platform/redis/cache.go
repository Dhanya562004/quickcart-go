package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Standard Redis key formatters for QuickCart.
func ProductKey(productID uuid.UUID) string {
	return fmt.Sprintf("quickcart:product:%s", productID.String())
}

func InventoryKey(storeID, productID uuid.UUID) string {
	return fmt.Sprintf("quickcart:inventory:%s:%s", storeID.String(), productID.String())
}

// GetOrSet implements the Cache-Aside pattern for any JSON-serializable type T.
// Resilient behavior:
// 1. Attempts to read from Redis. On cache hit, deserializes and returns.
// 2. On cache miss or if Redis is offline/failing: calls fetch() against PostgreSQL (source of truth).
// 3. If fetch() succeeds, asynchronously populates Redis with the specified TTL.
// 4. Redis failures NEVER cause the user request to fail; they log a warning and fall through to PostgreSQL.
func GetOrSet[T any](
	ctx context.Context,
	client *Client,
	key string,
	ttl time.Duration,
	fetch func(ctx context.Context) (T, error),
) (T, error) {
	// If Redis client is nil, immediately bypass to database
	if client == nil || client.rdb == nil {
		return fetch(ctx)
	}

	// 1. Try reading from cache
	val, err := client.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var cached T
		if jsonErr := json.Unmarshal(val, &cached); jsonErr == nil {
			return cached, nil
		}
		client.logger.Warn("failed to unmarshal cached value; falling back to DB", "key", key)
	} else if !errors.Is(err, redis.Nil) {
		// Log network or operational errors, but continue to DB
		client.logger.Warn("redis cache read error; falling through to database",
			"key", key,
			"error", err,
		)
	}

	// 2. Fetch from source of truth (PostgreSQL)
	data, fetchErr := fetch(ctx)
	if fetchErr != nil {
		var zero T
		return zero, fetchErr
	}

	// 3. Write back to Redis with TTL
	go func() {
		writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		bytes, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			client.logger.Warn("failed to marshal value for caching", "key", key, "error", marshalErr)
			return
		}

		if setErr := client.rdb.Set(writeCtx, key, bytes, ttl).Err(); setErr != nil {
			client.logger.Warn("failed to set cache value in redis", "key", key, "error", setErr)
		}
	}()

	return data, nil
}

// Invalidate removes one or more keys from Redis.
// Silently succeeds if Redis is offline so that database writes are not impeded.
func Invalidate(ctx context.Context, client *Client, keys ...string) error {
	if client == nil || client.rdb == nil || len(keys) == 0 {
		return nil
	}

	delCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := client.rdb.Del(delCtx, keys...).Err(); err != nil && !errors.Is(err, redis.Nil) {
		client.logger.Warn("cache invalidation failed in redis; continuing",
			"keys", keys,
			"error", err,
		)
		return nil
	}
	return nil
}
