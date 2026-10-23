package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID injects or propagates a unique X-Request-ID header and binds it to context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Header("X-Request-ID", reqID)
		ctx := logger.ContextWithRequestID(c.Request.Context(), reqID)
		c.Request = c.Request.WithContext(ctx)
		c.Set(string(logger.RequestIDKey), reqID)

		c.Next()
	}
}

// StructuredLogger logs HTTP request metadata and latency using structured slog.
func StructuredLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		reqID := c.GetString(string(logger.RequestIDKey))

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		reqLogger := log.With(
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
		)

		if status >= http.StatusInternalServerError {
			reqLogger.Error("HTTP request failed", slog.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		} else if status >= http.StatusBadRequest {
			reqLogger.Warn("HTTP client request error")
		} else {
			reqLogger.Info("HTTP request processed")
		}
	}
}

// Recovery recovers from unhandled panics and outputs a standardized RFC 7807 error payload.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				reqID := c.GetString(string(logger.RequestIDKey))
				stack := string(debug.Stack())

				log.Error("Unhandled panic in HTTP handler",
					slog.String("request_id", reqID),
					slog.Any("panic", p),
					slog.String("stack", stack),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "Internal Server Error",
					"message":    fmt.Sprintf("%v", p),
					"request_id": reqID,
				})
			}
		}()

		c.Next()
	}
}

// CORS applies essential cross-origin headers.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Idempotency-Key, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
