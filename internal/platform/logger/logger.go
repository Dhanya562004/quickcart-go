package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type contextKey string

const (
	// RequestIDKey is the context key for tracing HTTP request IDs.
	RequestIDKey contextKey = "request_id"
)

// New initializes a production-grade *slog.Logger based on environment and log level.
func New(env string, levelStr string, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}

	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: strings.ToLower(env) != "production",
	}

	var handler slog.Handler
	if strings.ToLower(env) == "production" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	return slog.New(handler)
}

// WithContext returns a logger annotated with any trace values (like request_id) found in ctx.
func WithContext(ctx context.Context, l *slog.Logger) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}

	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		return l.With(slog.String("request_id", reqID))
	}

	return l
}

// ContextWithRequestID attaches a request ID to a context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID retrieves the request ID from a context if present.
func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}
