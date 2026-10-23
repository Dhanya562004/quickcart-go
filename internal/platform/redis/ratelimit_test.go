package redis_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/platform/redis"
)

func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/0"
	}

	client, err := redis.NewClient(context.Background(), redisURL, nil)
	if err != nil || client.Ping(context.Background()) != nil {
		t.Skip("skipping Redis integration test; Redis is not reachable at " + redisURL)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	client := setupTestRedis(t)
	limiter := redis.NewRateLimiter(client)
	ctx := context.Background()

	id := "test-user-" + uuid.New().String()
	limit := 5
	window := 2 * time.Second

	for i := 0; i < limit; i++ {
		res, err := limiter.Allow(ctx, id, limit, window)
		require.NoError(t, err)
		assert.True(t, res.Allowed, "request %d should be allowed", i+1)
		assert.Equal(t, limit-i-1, res.Remaining)
	}

	// Next request must be rejected
	res, err := limiter.Allow(ctx, id, limit, window)
	require.NoError(t, err)
	assert.False(t, res.Allowed, "request exceeding limit must be blocked")
	assert.True(t, res.RetryAfter >= 1, "retry after must be at least 1 second")
}

func TestRateLimiter_UniqueMembersUnderConcurrency(t *testing.T) {
	client := setupTestRedis(t)
	limiter := redis.NewRateLimiter(client)
	ctx := context.Background()

	id := "concurrent-user-" + uuid.New().String()
	const (
		totalRequests = 20
		limit         = 10
	)
	window := 5 * time.Second

	var wg sync.WaitGroup
	var allowedCount atomic.Int32
	var blockedCount atomic.Int32

	startBarrier := make(chan struct{})

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier
			res, err := limiter.Allow(ctx, id, limit, window)
			if err == nil {
				if res.Allowed {
					allowedCount.Add(1)
				} else {
					blockedCount.Add(1)
				}
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	assert.Equal(t, int32(limit), allowedCount.Load(), "exactly %d requests must be allowed", limit)
	assert.Equal(t, int32(totalRequests-limit), blockedCount.Load(), "exactly %d requests must be rejected", totalRequests-limit)
}

func TestRateLimiter_Middleware_HTTP429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := setupTestRedis(t)
	limiter := redis.NewRateLimiter(client)

	r := gin.New()
	r.Use(limiter.Middleware(2, 5*time.Second))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	userID := uuid.New().String()

	// Req 1: OK
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-User-ID", userID)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Req 2: OK
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-User-ID", userID)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Req 3: Blocked (429)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.Header.Set("X-User-ID", userID)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	assert.NotEmpty(t, w3.Header().Get("Retry-After"))
}

func TestRateLimiter_FailOpenWhenNil(t *testing.T) {
	// A nil limiter must fail-open safely without panic
	var limiter *redis.RateLimiter
	res, err := limiter.Allow(context.Background(), "user-nil", 10, time.Minute)
	require.NoError(t, err)
	assert.True(t, res.Allowed, "nil limiter must fail-open")
}
