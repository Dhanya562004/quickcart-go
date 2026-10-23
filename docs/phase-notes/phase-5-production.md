# QuickCart Phase 5: Production Engineering, Security, & Operational Observability

## 1. Executive Summary

Phase 5 hardens QuickCart into a battle-tested, secure, observable, and production-ready quick-commerce backend. Building upon the concurrent engines and distributed infrastructure of Phases 1–4, Phase 5 introduces cryptographic authentication, role-based access control (RBAC), Server-Sent Events (SSE) security, strictly bounded Prometheus metrics, Grafana dashboard provisioning, complete OpenAPI 3.0 documentation with embedded Swagger UI, Testcontainers integration testing, and a comprehensive GitHub Actions CI pipeline.

### Core Architectural Invariants & Security Guarantees
1. **Zero-Trust Identity Derivation**: Client-supplied headers (e.g. `X-User-ID`, `X-User-Role`) and unauthenticated query parameters are **never trusted** for authorization. Identity and roles are strictly extracted and verified from HMAC-SHA256 signed JWT claims injected into the Gin context.
2. **Atomic Refresh Token Rotation (Anti-Replay)**: Refresh tokens are stored as SHA-256 hashes in PostgreSQL. Token rotation uses row-level locking (`SELECT ... FOR UPDATE`) inside an ACID transaction to atomically consume the token and mark it revoked before issuing a replacement pair, preventing concurrent token reuse and replay attacks.
3. **Dual-Transport SSE Security**: Solves the native browser `EventSource` limitation (inability to set custom `Authorization` request headers) by supporting verified query parameter tokens (`?token=<jwt>`) alongside standard `Authorization: Bearer <jwt>`, strictly validating order ownership against authenticated identity.
4. **Strictly Bounded Metric Cardinality**: Prometheus labels are limited to fixed, low-cardinality dimensions (`method`, `route_pattern`, `status_code`, `outcome`, `queue_type`, `pool`). User IDs, order IDs, and arbitrary inputs are never used as labels, preventing memory exhaustion and metric blowup.
5. **HMAC-Protected Payment Gateway Webhooks**: Webhook callbacks require an `X-Gateway-Signature` header computed via HMAC-SHA256 of the raw payload and validated using constant-time comparison (`hmac.Equal`).
6. **Containerized Integration Testing with Automated Migrations**: Testcontainers runs PostgreSQL 16 and Redis 7 with embedded schema migrations applied automatically. In CI (`CI=true`), tests fail explicitly if Docker is unavailable—no silent skipping.

---

## 2. Deep Dive: "Why This Design?"

### A. Authentication & Atomic Refresh Token Rotation (`internal/auth/`)
- **Password Security**: Passwords are validated for complexity (minimum 8 characters, alphanumeric + symbol) and hashed using `bcrypt` at cost 12. Credentials and raw tokens are never written to structured logs.
- **Short-Lived Access Tokens**: Signed using HMAC-SHA256 (`jwt.SigningMethodHS256`) with a 15-minute TTL to minimize exposure window upon leak.
- **Hashed Refresh Tokens**: 32-byte cryptographically secure random tokens (`crypto/rand`) hashed via SHA-256 before storage in the `refresh_tokens` table. Even if the database is dumped, tokens cannot be replayed.
- **Atomic Rotation Query**:
  ```sql
  SELECT id, user_id, token_hash, expires_at, revoked, created_at, updated_at
  FROM refresh_tokens
  WHERE token_hash = $1
    AND revoked = FALSE
    AND expires_at > NOW()
  FOR UPDATE;
  ```
  The row lock serializes concurrent rotation requests for the same token. The first request updates `revoked = TRUE` and commits; any concurrent duplicate request receives an empty row and is rejected with HTTP 401 Unauthorized.

### B. SSE Order Tracking Authentication (`internal/tracking/`)
- **Problem**: The browser's native `EventSource` API does not allow setting custom HTTP headers (such as `Authorization: Bearer <jwt>`).
- **Solution**: The `auth.Authenticate` middleware accepts tokens from two valid transports:
  1. Header: `Authorization: Bearer <token>` (cURL, mobile clients, SDKs)
  2. Query Param: `?token=<token>` (Native Browser `EventSource`)
- **Ownership Verification**: Before establishing the long-lived streaming connection, the handler verifies:
  ```go
  if role != auth.RoleAdmin && ord.UserID != callerUserID {
      c.JSON(http.StatusForbidden, gin.H{"error": "forbidden; you do not own this order"})
      return
  }
  ```
- **Goroutine Leak Protection**: Connections monitor `c.Request.Context().Done()`. When a client disconnects or the server shuts down, the client channel is unregistered from `SSEHub` and closed, verified leak-free via `goleak.VerifyNone`.

### C. Bounded-Label Observability (`internal/platform/metrics/`)
- **Label Explosion Risk**: Placing dynamic identifiers (like `order_id` or `user_id`) in Prometheus metrics causes the time-series database to grow unbounded, leading to high memory consumption and scrape failures.
- **Solution**:
  - `quickcart_http_requests_total{method, route, status}` — `route` uses normalized Gin route templates (e.g. `/api/v1/orders/:id`), not dynamic UUIDs.
  - `quickcart_http_request_duration_seconds{method, route}` — Exponential latency histogram buckets: `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]`.
  - `quickcart_concurrency_queue_depth{queue_type}`
  - `quickcart_concurrency_worker_active_count{pool}`
  - `quickcart_inventory_reservation_outcomes_total{outcome}` — outcomes: `reserved`, `rejected`, `released`, `expired`.
  - `quickcart_payment_attempts_total{status, method}` — statuses: `success`, `transient_fail`, `terminal_fail`.
- **Metrics Protection**: In production (`APP_ENV=production`), `GET /metrics` requires a valid Bearer scrape token (`METRICS_TOKEN`) or an authenticated Administrator JWT.

### D. OpenAPI 3.0 & Swagger UI (`api/`)
- Complete OpenAPI 3.0.3 specification in `api/openapi.yaml`, embedded into the Go binary using `//go:embed`.
- Interactive Swagger UI served at `GET /docs` without external file dependencies.
- Raw YAML specification available at `GET /api/openapi.yaml`.

### E. Testcontainers Integration Suite (`internal/testutil/`)
- Dedicated PostgreSQL 16 and Redis 7 containers spun up on-demand.
- Schema migrations (`000001_init_schema.up.sql` and `000002_auth_refresh_tokens.up.sql`) applied automatically during container bootstrap.
- Proper container isolation and guaranteed teardown in `defer cleanup()`.
- Enforces strict CI execution: fails the build if Docker is not available in CI, avoiding accidental skips.

### F. GitHub Actions CI Pipeline (`.github/workflows/ci.yml`)
- Triggers on push to `main` and pull requests targeting `main`.
- Jobs:
  1. `lint`: Go vet, `golangci-lint` (5-minute timeout).
  2. `test`: `go test -v -race -timeout 10m ./...` (verifies race-free execution across all packages).
  3. `integration`: Runs Testcontainers against real PostgreSQL and Redis containers.
  4. `compose`: Validates Docker Compose service definitions and syntax.

---

## 3. Production Deployment & Monitoring

### Docker Compose Stack
The compose stack (`deployments/docker-compose.yml`) provisions:
- `quickcart-db`: PostgreSQL 16 Alpine with initialized schemas
- `quickcart-redis`: Redis 7 Alpine
- `quickcart-api`: QuickCart Go Engine (port 8080)
- `quickcart-prometheus`: Prometheus v2.51.0 scraping API metrics with scrape token (port 9090)
- `quickcart-grafana`: Grafana 10.4.0 with provisioned Prometheus datasource and pre-configured dashboard (port 3000)

---

## 4. Verification Suite Results

| Test Suite | Package | Race Detector (`-race`) | Result |
| :--- | :--- | :--- | :--- |
| **Authentication & RBAC** | `internal/auth` | Enabled | **PASS** (100% pass, zero leaks) |
| **Password Leaks Check** | `internal/auth` | Enabled | **PASS** (no passwords/tokens in slog) |
| **Token Expiry Mock Clock** | `internal/auth` | Enabled | **PASS** (clock advanced 16m without sleep) |
| **SSE Security & Ownership** | `internal/tracking` | Enabled | **PASS** (zero leaks via `goleak`) |
| **Payment Security & HMAC** | `internal/payment` | Enabled | **PASS** (HMAC verified, stable idempotency) |
| **Metrics Middleware** | `internal/platform/metrics`| Enabled | **PASS** (bounded labels, no UUIDs) |
| **Server & Routing** | `cmd/server` | Enabled | **PASS** (docs, metrics auth, health) |
| **Testcontainers Suite** | `internal/testutil` | Enabled | **PASS** (auto-migrations, clean cleanup) |
