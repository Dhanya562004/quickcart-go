package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Dhanya562004/quickcart-go/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// SetupPostgresContainer initializes a dedicated PostgreSQL 16-alpine container,
// executes schema migrations automatically, and provides an isolated test database.
//
// Enforcement Invariant:
// In CI environments (os.Getenv("CI") != ""), tests will NEVER silently skip if Docker
// is unavailable; they fail explicitly. In local dev environments without Docker,
// they informatively skip with prerequisite instructions.
func SetupPostgresContainer(ctx context.Context, t testing.TB) (string, func()) {
	t.Helper()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("quickcart_test"),
		tcpostgres.WithUsername("quickcart"),
		tcpostgres.WithPassword("quickcart_test_secret"),
	)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Testcontainers failure: Docker is required in CI but failed to start PostgreSQL 16 container: %v", err)
		}
		t.Skipf("Skipping integration test: Docker prerequisite not met (Docker Desktop / Docker daemon required): %v", err)
		return "", func() {}
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("Failed to retrieve PostgreSQL container connection string: %v", err)
	}

	// Apply migrations automatically
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("Failed to connect to PostgreSQL container: %v", err)
	}

	if _, err := pool.Exec(ctx, migrations.SchemaV1); err != nil {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("Failed to execute Migration V1 (init schema): %v", err)
	}

	if _, err := pool.Exec(ctx, migrations.SchemaV2); err != nil {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("Failed to execute Migration V2 (auth refresh tokens): %v", err)
	}
	pool.Close()

	cleanup := func() {
		termCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := pgContainer.Terminate(termCtx); err != nil {
			t.Logf("Warning: failed to terminate PostgreSQL container: %v", err)
		}
	}

	return connStr, cleanup
}

// SetupRedisContainer initializes a dedicated Redis 7-alpine container and returns the connection URL.
func SetupRedisContainer(ctx context.Context, t testing.TB) (string, func()) {
	t.Helper()

	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Testcontainers failure: Docker is required in CI but failed to start Redis 7 container: %v", err)
		}
		t.Skipf("Skipping integration test: Docker prerequisite not met (Docker Desktop / Docker daemon required): %v", err)
		return "", func() {}
	}

	endpoint, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		_ = redisContainer.Terminate(ctx)
		t.Fatalf("Failed to retrieve Redis container connection string: %v", err)
	}

	cleanup := func() {
		termCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := redisContainer.Terminate(termCtx); err != nil {
			t.Logf("Warning: failed to terminate Redis container: %v", err)
		}
	}

	return endpoint, cleanup
}
