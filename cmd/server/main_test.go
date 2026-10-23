package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSetupRouter_Healthz(t *testing.T) {
	cfg := &config.Config{
		Environment: config.EnvDevelopment,
		LogLevel:    "info",
		ServerPort:  "8080",
	}

	router := SetupRouter(cfg, getTestLogger(), nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "alive", body["status"])
	assert.NotEmpty(t, body["timestamp"])
}

func TestSetupRouter_Readyz_NotReady(t *testing.T) {
	cfg := &config.Config{
		Environment: config.EnvDevelopment,
		LogLevel:    "info",
		ServerPort:  "8080",
	}

	// dbConn is nil -> readyz should return 503 Service Unavailable
	router := SetupRouter(cfg, getTestLogger(), nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "not_ready", body["status"])
	assert.Equal(t, "disconnected", body["database"])
}

func TestSetupRouter_SystemInfo(t *testing.T) {
	cfg := &config.Config{
		Environment: config.EnvStaging,
		LogLevel:    "info",
		ServerPort:  "8080",
	}

	router := SetupRouter(cfg, getTestLogger(), nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "QuickCart Concurrent Engine", body["service"])
	assert.Equal(t, Version, body["version"])
	assert.Equal(t, "staging", body["environment"])
}

func TestSetupRouter_NotFound(t *testing.T) {
	cfg := &config.Config{
		Environment: config.EnvDevelopment,
		LogLevel:    "info",
		ServerPort:  "8080",
	}

	router := SetupRouter(cfg, getTestLogger(), nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/unknown-endpoint-404", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestSetupRouter_RateLimiting_FailOpen_WhenRedisNil(t *testing.T) {
	cfg := &config.Config{
		Environment:       config.EnvDevelopment,
		LogLevel:          "info",
		ServerPort:        "8080",
		RateLimitRequests: 10,
		RateLimitWindow:   1 * time.Minute,
	}

	// With nil Redis client, rate limiter is bypassed and system remains fully responsive
	router := SetupRouter(cfg, getTestLogger(), nil, nil, nil, nil)

	for i := 0; i < 15; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}
