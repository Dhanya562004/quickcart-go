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

	// Use BasicWaitStrategies which waits for PostgreSQL to log "database system is ready to accept connections"
	// twice (handling the initial entrypoint bootstrap restart) and for the port to be open.
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("quickcart_test"),
		tcpostgres.WithUsername("quickcart"),
		tcpostgres.WithPassword("quickcart_test_secret"),
		tcpostgres.BasicWaitStrategies(),
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

	// Verify PostgreSQL container is genuinely ready to accept queries.
	// Alpine PostgreSQL entrypoint initializes the cluster, restarts the daemon,
	// and may briefly reset early connections before becoming stable.
	var pool *pgxpool.Pool
	var pingErr error
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		pool, pingErr = pgxpool.New(ctx, connStr)
		if pingErr == nil {
			pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
			pingErr = pool.Ping(pingCtx)
			pingCancel()
			if pingErr == nil {
				var one int
				queryCtx, queryCancel := context.WithTimeout(ctx, 2*time.Second)
				pingErr = pool.QueryRow(queryCtx, "SELECT 1").Scan(&one)
				queryCancel()
				if pingErr == nil && one == 1 {
					break
				}
			}
			pool.Close()
			pool = nil
		}

		select {
		case <-ctx.Done():
			_ = pgContainer.Terminate(context.Background())
			t.Fatalf("Context cancelled while waiting for PostgreSQL readiness: %v", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}

	if pingErr != nil || pool == nil {
		_ = pgContainer.Terminate(context.Background())
		t.Fatalf("PostgreSQL container did not become ready within timeout: %v", pingErr)
	}

	// Apply migrations automatically with retry for resilience
	var migErr error
	for attempt := 1; attempt <= 5; attempt++ {
		execCtx, execCancel := context.WithTimeout(ctx, 5*time.Second)
		_, migErr = pool.Exec(execCtx, migrations.SchemaV1)
		execCancel()
		if migErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if migErr != nil {
		pool.Close()
		_ = pgContainer.Terminate(context.Background())
		t.Fatalf("Failed to execute Migration V1 (init schema): %v", migErr)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		execCtx, execCancel := context.WithTimeout(ctx, 5*time.Second)
		_, migErr = pool.Exec(execCtx, migrations.SchemaV2)
		execCancel()
		if migErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if migErr != nil {
		pool.Close()
		_ = pgContainer.Terminate(context.Background())
		t.Fatalf("Failed to execute Migration V2 (auth refresh tokens): %v", migErr)
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
