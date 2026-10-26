package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dhanya562004/quickcart-go/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestIDMiddleware_Generated(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())

	var capturedReqID string
	r.GET("/test", func(c *gin.Context) {
		capturedReqID = logger.GetRequestID(c.Request.Context())
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	respReqID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, respReqID)
	assert.Equal(t, respReqID, capturedReqID)
}

func TestRequestIDMiddleware_Preserved(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())

	customID := "custom-trace-uuid-12345"
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", customID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, customID, w.Header().Get("X-Request-ID"))
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(RequestID())
	r.Use(Recovery(testLogger))

	r.GET("/panic", func(c *gin.Context) {
		panic("database connection blew up")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Internal Server Error", resp["error"])
	assert.Contains(t, resp["message"], "database connection blew up")
	assert.NotEmpty(t, resp["request_id"])
}

func TestStructuredLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(RequestID())
	r.Use(StructuredLogger(testLogger))

	r.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "world"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/hello?foo=bar", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, buf.String(), "/hello?foo=bar")
	assert.Contains(t, buf.String(), "HTTP request processed")
}

func TestCORSMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Preflight OPTIONS without Origin (e.g. basic tool)
	wOpt := httptest.NewRecorder()
	reqOpt, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
	r.ServeHTTP(wOpt, reqOpt)
	assert.Equal(t, http.StatusNoContent, wOpt.Code)
	assert.Equal(t, "*", wOpt.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, wOpt.Header().Get("Access-Control-Allow-Credentials")) // Never wildcard + credentials

	// Actual GET without Origin
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	assert.Equal(t, "*", wGet.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, wGet.Header().Get("Access-Control-Allow-Credentials"))

	// Preflight with Origin echoes origin and permits credentials
	wOptOrigin := httptest.NewRecorder()
	reqOptOrigin, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
	reqOptOrigin.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(wOptOrigin, reqOptOrigin)
	assert.Equal(t, http.StatusNoContent, wOptOrigin.Code)
	assert.Equal(t, "http://localhost:5173", wOptOrigin.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", wOptOrigin.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Origin", wOptOrigin.Header().Get("Vary"))
	assert.Contains(t, wOptOrigin.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, wOptOrigin.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key")
}

func TestCORSMiddleware_ScopedOrigins(t *testing.T) {
	r := gin.New()
	r.Use(CORS("https://quickcart.app", "http://localhost:5173"))
	r.GET("/items", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Allowed origin 1
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/items", nil)
	req1.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "http://localhost:5173", w1.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w1.Header().Get("Access-Control-Allow-Credentials"))

	// Allowed origin 2
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/items", nil)
	req2.Header.Set("Origin", "https://quickcart.app")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "https://quickcart.app", w2.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w2.Header().Get("Access-Control-Allow-Credentials"))

	// Disallowed origin
	wDisallowed := httptest.NewRecorder()
	reqDisallowed, _ := http.NewRequest(http.MethodGet, "/items", nil)
	reqDisallowed.Header.Set("Origin", "https://malicious-site.com")
	r.ServeHTTP(wDisallowed, reqDisallowed)
	assert.Equal(t, http.StatusOK, wDisallowed.Code)
	assert.Empty(t, wDisallowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, wDisallowed.Header().Get("Access-Control-Allow-Credentials"))
}

