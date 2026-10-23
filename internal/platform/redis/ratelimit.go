package redis

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// slidingWindowLua is an atomic Lua script executing a sliding-window log rate limiter.
// Guarantees:
// 1. Atomic execution in Redis single-threaded engine (no check-then-set race conditions).
// 2. Uses unique sorted-set members (timestamp:randomUUID) to prevent member overwrite collisions
//    when multiple concurrent requests arrive at the identical microsecond.
// 3. Returns {allowed (1 or 0), remaining, retry_after_seconds}.
const slidingWindowLua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local memberId = ARGV[4]
local clearBefore = now - window

-- Remove requests older than the sliding window boundary
redis.call('ZREMRANGEBYSCORE', key, '-inf', clearBefore)
local currentRequests = redis.call('ZCARD', key)

if currentRequests < limit then
    -- Record this request with a unique memberId
    redis.call('ZADD', key, now, memberId)
    -- Set TTL slightly larger than window duration in seconds
    redis.call('EXPIRE', key, math.ceil(window / 1000000) + 1)
    return {1, limit - currentRequests - 1, 0}
else
    -- Find oldest request timestamp in window to compute retry-after
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retryAfter = 1
    if oldest and #oldest >= 2 then
        local oldestScore = tonumber(oldest[2])
        if oldestScore then
            retryAfter = math.ceil((oldestScore + window - now) / 1000000)
        end
    end
    if retryAfter < 1 then retryAfter = 1 end
    return {0, 0, retryAfter}
end
`

// RateLimitResult encapsulates the rate-limiter decision.
type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter int // in seconds
}

// RateLimiter manages distributed rate limiting using Redis.
type RateLimiter struct {
	client *Client
	script *redis.Script
}

// NewRateLimiter initializes a RateLimiter instance with pre-loaded Lua script.
func NewRateLimiter(client *Client) *RateLimiter {
	return &RateLimiter{
		client: client,
		script: redis.NewScript(slidingWindowLua),
	}
}

// Allow evaluates if a request identified by identifier is within the rate limit.
// If Redis is unreachable, it defaults to FAIL-OPEN (permits request, logs warning)
// to prevent bringing down API availability due to cache issues.
func (r *RateLimiter) Allow(
	ctx context.Context,
	identifier string,
	limit int,
	window time.Duration,
) (RateLimitResult, error) {
	if r == nil || r.client == nil || r.client.rdb == nil {
		// Redis offline: fail-open
		return RateLimitResult{Allowed: true, Remaining: limit, RetryAfter: 0}, nil
	}

	key := fmt.Sprintf("quickcart:ratelimit:%s", identifier)
	nowMicros := time.Now().UnixNano() / 1000
	windowMicros := window.Microseconds()
	uniqueMember := fmt.Sprintf("%d:%s", nowMicros, uuid.New().String())

	evalCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	res, err := r.script.Run(evalCtx, r.client.rdb, []string{key}, nowMicros, windowMicros, limit, uniqueMember).Result()
	if err != nil {
		r.client.logger.Warn("rate limiter redis evaluation error; failing open",
			"identifier", identifier,
			"error", err,
		)
		return RateLimitResult{Allowed: true, Remaining: limit, RetryAfter: 0}, nil
	}

	vals, ok := res.([]any)
	if !ok || len(vals) < 3 {
		return RateLimitResult{Allowed: true, Remaining: limit, RetryAfter: 0}, nil
	}

	allowedInt, _ := vals[0].(int64)
	remainingInt, _ := vals[1].(int64)
	retryAfterInt, _ := vals[2].(int64)

	return RateLimitResult{
		Allowed:    allowedInt == 1,
		Remaining:  int(remainingInt),
		RetryAfter: int(retryAfterInt),
	}, nil
}

// Middleware creates a Gin middleware enforcing the specified rate limit per user/IP.
func (r *RateLimiter) Middleware(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Identify user by X-User-ID if present; otherwise fallback to Client IP
		var identifier string
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			identifier = "user:" + userID
		} else {
			identifier = "ip:" + c.ClientIP()
		}

		res, _ := r.Allow(c.Request.Context(), identifier, limit, window)

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

		if !res.Allowed {
			c.Header("Retry-After", strconv.Itoa(res.RetryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded; please slow down",
				"retry_after": res.RetryAfter,
			})
			return
		}

		c.Next()
	}
}
