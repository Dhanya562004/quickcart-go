package database

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Dhanya562004/quickcart-go/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short url",
			input:    "postgres://localhost",
			expected: "postgres://localhost",
		},
		{
			name:     "long credentials url",
			input:    "postgres://user:supersecretpass@localhost:5432/quickcart?sslmode=disable",
			expected: "postgres://user...sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sanitizeURL(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestWithTx_NilPool(t *testing.T) {
	ctx := context.Background()
	err := WithTx(ctx, nil, func(tx pgx.Tx) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database pool is nil")
}

func TestDB_Ping_NilPool(t *testing.T) {
	db := &DB{Pool: nil, logger: slog.Default()}
	err := db.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool is not initialized")
}

func TestNewPool_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "::invalid-url::",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := NewPool(context.Background(), cfg, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse database configuration")
}

func TestNewPool_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping integration test; DATABASE_URL not set")
	}
	cfg := &config.Config{
		DatabaseURL:        dbURL,
		DBMaxConns:         5,
		DBMinConns:         1,
		DBHealthCheckDelay: time.Minute,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool, err := NewPool(context.Background(), cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	err = pool.Ping(context.Background())
	require.NoError(t, err)
}

