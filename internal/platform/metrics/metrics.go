package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP Metrics
	// Critical Invariant: Labels are strictly bounded to method, route, and status.
	// User IDs, Order IDs, and arbitrary inputs are NEVER used as labels to prevent memory exhaustion.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "quickcart",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed, partitioned by method, route, and HTTP status code.",
		},
		[]string{"method", "route", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "quickcart",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request latency distributions partitioned by method and route pattern.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)

	// Concurrency & Queue Metrics
	queueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "quickcart",
			Subsystem: "concurrency",
			Name:      "queue_depth",
			Help:      "Current number of pending jobs in background worker queues.",
		},
		[]string{"queue_type"},
	)

	activeWorkers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "quickcart",
			Subsystem: "concurrency",
			Name:      "worker_active_count",
			Help:      "Number of worker pool goroutines actively executing processing tasks.",
		},
		[]string{"pool"},
	)

	// Business & Inventory Metrics
	reservationOutcomes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "quickcart",
			Subsystem: "inventory",
			Name:      "reservation_outcomes_total",
			Help:      "Total inventory reservation outcomes partitioned by result state (reserved, rejected, released, expired).",
		},
		[]string{"outcome"},
	)

	paymentAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "quickcart",
			Subsystem: "payment",
			Name:      "attempts_total",
			Help:      "Total payment transaction execution attempts by result status and payment method.",
		},
		[]string{"status", "method"},
	)
)

// RecordHTTPRequest updates Prometheus counters and latency histograms for an HTTP interaction.
func RecordHTTPRequest(method, route string, status int, duration time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	httpRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// SetQueueDepth updates the gauge for pending queue items.
func SetQueueDepth(queueType string, depth float64) {
	queueDepth.WithLabelValues(queueType).Set(depth)
}

// SetActiveWorkers updates the gauge tracking busy worker goroutines.
func SetActiveWorkers(pool string, count float64) {
	activeWorkers.WithLabelValues(pool).Set(count)
}

// RecordReservationOutcome increments counters for inventory stock allocation states.
// Valid outcomes: "reserved", "rejected", "released", "expired".
func RecordReservationOutcome(outcome string) {
	reservationOutcomes.WithLabelValues(outcome).Inc()
}

// RecordPaymentAttempt tracks payment authorization and capture outcomes.
// Valid statuses: "success", "transient_fail", "terminal_fail".
// Valid methods: "UPI", "CARD", "NETBANKING", "WALLET".
func RecordPaymentAttempt(status, method string) {
	paymentAttempts.WithLabelValues(status, method).Inc()
}

// Middleware creates a Gin middleware collecting latency and status metrics with normalized route patterns.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// Use matched route pattern (e.g. /api/v1/orders/:id) instead of raw path with dynamic UUIDs
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		RecordHTTPRequest(c.Request.Method, route, c.Writer.Status(), time.Since(start))
	}
}

// Handler returns standard Prometheus metrics endpoint handler.
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// AuthMiddleware protects the /metrics endpoint against unauthorized public scraping in production.
// It permits requests if:
// 1. Environment is development/staging and no token is enforced, OR
// 2. Authorization header matches "Bearer <expectedToken>", OR
// 3. User is an authenticated Administrator.
func AuthMiddleware(expectedToken string, env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If in dev and no token specified, permit access for ease of local testing
		if env != "production" && expectedToken == "" {
			c.Next()
			return
		}

		// Check Bearer token
		authHeader := c.GetHeader("Authorization")
		if expectedToken != "" && authHeader == "Bearer "+expectedToken {
			c.Next()
			return
		}

		// Check admin role if request was previously authenticated
		if role, exists := c.Get("user_role"); exists && role == "admin" {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(401, gin.H{
			"error": "unauthorized metrics access",
		})
	}
}
