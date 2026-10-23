# QuickCart — Concurrent Quick-Commerce Engine

[![Go Report Card](https://goreportcard.com/badge/github.com/Dhanya562004/quickcart-go)](https://goreportcard.com/report/github.com/Dhanya562004/quickcart-go)
[![CI](https://github.com/Dhanya562004/quickcart-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Dhanya562004/quickcart-go/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

QuickCart is a high-throughput, concurrent quick-commerce order and inventory management engine in **Go 1.25**, inspired by the operational demands of **Swiggy Instamart, Blinkit, and Zepto**. It is engineered to satisfy sub-10-minute delivery SLAs under extreme concurrent flash-sale traffic, providing mathematical guarantees of zero inventory overselling, atomic token rotation, and low-latency order tracking.

---

## Architecture & Core Features

```
                                  +-----------------------+
                                  |   Client Applications |
                                  +-----------+-----------+
                                              |
                          (JWT / ?token= / Idempotency-Key)
                                              |
                                              v
                                  +-----------------------+
                                  |     Gin API Engine    |
                                  |  - Request ID / CORS  |
                                  |  - Auth & RBAC        |
                                  |  - Bounded Metrics    |
                                  +-----+-----------+-----+
                                        |           |
            +---------------------------+           +---------------------------+
            | (Read-Through Cache / Lua Rate Limit) | (ACID Transactions / Locks)
            v                                       v
+-----------------------+               +-----------------------+
|        Redis 7        |               |     PostgreSQL 16     |
| - Cache-aside catalog |               | - Inventory FOR UPDATE|
| - Sliding-window limit|               | - SKIP LOCKED sweep   |
+-----------------------+               | - Atomic token rot.   |
                                        +-----------+-----------+
                                                    |
                                                    v
                                        +-----------------------+
                                        |  Background Workers   |
                                        | - Order confirm pool  |
                                        | - Expiry reclaimer    |
                                        | - SSE live tracking   |
                                        +-----------------------+
```

### 1. Concurrency & Inventory Guarantees
- **Zero-Overselling Inventory Locking**: Checkout acquires row-level locks on `store_inventories` in deterministic sorted product order, eliminating deadlocks while ensuring atomic reservation deduction.
- **PostgreSQL FOR UPDATE SKIP LOCKED**: Background expiry sweeps reclaim unfulfilled 10-minute reservations without lock contention or deadlocks across distributed worker instances.
- **Channel Queue & Worker Pool**: Generic worker pool with fan-out/fan-in workers, context cancellation, and bounded concurrency.

### 2. Distributed Resilience
- **Cache-Aside with Transparent Fallback**: Product catalog and inventory lookups query Redis first with a 1-hour TTL and transparently fall back to PostgreSQL on cache miss or Redis disruption.
- **Collision-Free Sliding-Window Rate Limiting**: Atomic Redis Lua script using microsecond composite members (`timestamp:uuid`) prevents counter collisions during high-concurrency bursts.
- **Idempotent Payment Retries**: Exponential backoff with full jitter using stable idempotency keys (`pay_order_<order_id>`) prevents duplicate charges across gateway retries.

### 3. Production Security & RBAC
- **Zero-Trust Identity**: Never trusts client headers (`X-User-ID`, `X-User-Role`) or unauthenticated query params. Identity is strictly derived from verified HMAC-SHA256 JWT claims.
- **Atomic Refresh Token Rotation**: Refresh tokens are stored as SHA-256 hashes. Single-use rotation uses row locks to prevent replay attacks.
- **Secure SSE Order Tracking**: Supports native browser `EventSource` via `?token=<jwt>` with strict ownership verification and clean client disconnection (verified leak-free via `goleak`).
- **HMAC Webhook Signatures**: Payment gateway callbacks verify HMAC-SHA256 signatures with constant-time comparison (`hmac.Equal`).

### 4. Observability & API Documentation
- **Bounded-Label Prometheus Metrics**: Strict control over label cardinality. Labels are restricted to static dimensions (`method`, `route_pattern`, `status`); dynamic user and order IDs are never exposed as labels.
- **Auto-Provisioned Grafana Dashboard**: Pre-configured Prometheus datasource and QuickCart overview dashboard tracking RED metrics, queue depth, inventory outcomes, and payment attempts.
- **Interactive OpenAPI 3.0 / Swagger UI**: Full API specification embedded in the binary and served interactively at `GET /docs`.

---

## Getting Started

### Prerequisites
- **Go**: 1.25+
- **Docker & Docker Compose**: (Required for full stack and Testcontainers)

### Running Locally with Docker Compose

To boot the entire production stack (PostgreSQL, Redis, QuickCart API, Prometheus, and Grafana):

```bash
docker compose -f deployments/docker-compose.yml up --build -d
```

| Service | Port | Description |
| :--- | :--- | :--- |
| **QuickCart API** | `8080` | REST API, SSE Streaming (`http://localhost:8080`) |
| **Swagger UI** | `8080` | Interactive Documentation (`http://localhost:8080/docs`) |
| **OpenAPI Spec** | `8080` | Raw OpenAPI 3.0 Spec (`http://localhost:8080/api/openapi.yaml`) |
| **PostgreSQL 16** | `5433` | Primary Relational Database |
| **Redis 7** | `6379` | Cache-Aside & Rate Limiter |
| **Prometheus** | `9090` | Metrics Scraper (`http://localhost:9090`) |
| **Grafana** | `3000` | Dashboards (`http://localhost:3000`, user: `admin`, pass: `admin`) |

---

## API Documentation

Interactive Swagger UI documentation is available at:
```
http://localhost:8080/docs
```

### Key Endpoints

| Category | Method | Path | Auth / Role | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Health** | `GET` | `/healthz` | Public | Process liveness check |
| **Health** | `GET` | `/readyz` | Public | Dependency readiness check (DB & Redis) |
| **Metrics** | `GET` | `/metrics` | Bearer Token / Admin | Bounded Prometheus metrics |
| **Auth** | `POST` | `/api/v1/auth/register` | Public | Register customer account |
| **Auth** | `POST` | `/api/v1/auth/login` | Public | Login and obtain JWT token pair |
| **Auth** | `POST` | `/api/v1/auth/refresh` | Public | Atomic refresh token rotation |
| **Auth** | `POST` | `/api/v1/auth/logout` | Public | Revoke refresh token |
| **Auth** | `GET` | `/api/v1/auth/me` | Customer / Admin | Get authenticated profile |
| **Catalog** | `GET` | `/api/v1/stores` | Public | List active dark stores |
| **Catalog** | `GET` | `/api/v1/stores/:id/products` | Public | List store inventory products |
| **Admin** | `POST` | `/api/v1/admin/products` | Admin Only | Create catalog product |
| **Admin** | `POST` | `/api/v1/admin/stores/:id/inventory` | Admin Only | Upsert store inventory stock |
| **Cart** | `GET` | `/api/v1/cart` | Customer | Get customer shopping cart |
| **Cart** | `POST` | `/api/v1/cart/items` | Customer | Add product to cart |
| **Orders** | `POST` | `/api/v1/checkout` | Customer | Atomic checkout with Idempotency-Key |
| **Orders** | `GET` | `/api/v1/orders/:id` | Owner / Admin | Get order details |
| **Orders** | `POST` | `/api/v1/orders/:id/cancel` | Owner / Admin | Cancel order and restore stock |
| **Payments**| `POST` | `/api/v1/orders/:id/pay` | Owner / Admin | Process payment with retry schedules |
| **Payments**| `POST` | `/api/v1/orders/:id/refund` | Admin Only | Process idempotent refund |
| **Payments**| `POST` | `/api/v1/payments/webhook` | HMAC Signed | Gateway callback verification |
| **Tracking**| `GET` | `/api/v1/orders/:id/events` | Owner / Admin | Real-time SSE order status stream |

---

## Testing & Quality Assurance

QuickCart enforces strict test quality without race conditions:

```bash
# Verify module checksums
go mod verify

# Compile all binaries
go build ./...

# Run static analysis and vetting
go vet ./...
golangci-lint run

# Run complete test suite with Go Race Detector
go test -v -race ./...

# Run Testcontainers Integration Suite (PostgreSQL 16 & Redis 7)
go test -v -race ./internal/testutil/...
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
