package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Dhanya562004/quickcart-go/internal/platform/metrics"
)

func TestMetrics_MiddlewareAndCollection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(metrics.Middleware())

	r.GET("/api/v1/orders/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", metrics.Handler())

	// 1. Send request with dynamic ID in path
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/11111111-2222-3333-4444-555555555555", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Record business domain metrics
	metrics.RecordReservationOutcome("reserved")
	metrics.RecordReservationOutcome("rejected")
	metrics.RecordPaymentAttempt("success", "UPI")
	metrics.SetQueueDepth("order_confirm", 5)
	metrics.SetActiveWorkers("main_pool", 4)

	// 3. Scrape /metrics endpoint
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsW := httptest.NewRecorder()
	r.ServeHTTP(metricsW, metricsReq)

	assert.Equal(t, http.StatusOK, metricsW.Code)
	body := metricsW.Body.String()

	// Assert: Metrics exist in Prometheus format
	assert.Contains(t, body, "quickcart_http_requests_total")
	assert.Contains(t, body, `route="/api/v1/orders/:id"`)
	// Invariant Check: The dynamic UUID must NOT appear as a label
	assert.NotContains(t, body, `11111111-2222-3333-4444-555555555555`)

	assert.Contains(t, body, "quickcart_inventory_reservation_outcomes_total")
	assert.Contains(t, body, `outcome="reserved"`)
	assert.Contains(t, body, "quickcart_concurrency_queue_depth")
	assert.Contains(t, body, "quickcart_payment_attempts_total")
}

func TestMetrics_RecordHTTPRequest_Direct(t *testing.T) {
	metrics.RecordHTTPRequest(http.MethodPost, "/api/v1/checkout", http.StatusCreated, 15*time.Millisecond)
	metrics.RecordHTTPRequest(http.MethodPost, "/api/v1/checkout", http.StatusConflict, 5*time.Millisecond)
}
