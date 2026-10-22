package middleware

import (
	"bytes"
	"encoding/json"
	"io"
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

	// Preflight OPTIONS
	wOpt := httptest.NewRecorder()
	reqOpt, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
	r.ServeHTTP(wOpt, reqOpt)
	assert.Equal(t, http.StatusNoContent, wOpt.Code)
	assert.Equal(t, "*", wOpt.Header().Get("Access-Control-Allow-Origin"))

	// Actual GET
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	assert.Equal(t, "*", wGet.Header().Get("Access-Control-Allow-Origin"))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
