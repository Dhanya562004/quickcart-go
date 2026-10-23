# QuickCart — Concurrent Quick-Commerce Order & Inventory Engine

[![Go Report Card](https://goreportcard.com/badge/github.com/Dhanya562004/quickcart-go)](https://goreportcard.com/report/github.com/Dhanya562004/quickcart-go)
[![CI](https://github.com/Dhanya562004/quickcart-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Dhanya562004/quickcart-go/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

QuickCart is a high-throughput, concurrent quick-commerce order and inventory management engine in **Go 1.25**, inspired by the engineering demands of **Swiggy Instamart, Blinkit, and Zepto**. It is engineered to satisfy sub-10-minute delivery SLAs under extreme concurrent flash-sale traffic, providing mathematical guarantees of zero inventory overselling, atomic token rotation, and low-latency order tracking.

---

## Distributed System Architecture

QuickCart runs as a horizontally-scaled microservice cluster consisting of an **Nginx ingress load balancer**, **three stateless Go API replicas**, a **shared PostgreSQL 16 database**, and a **Redis 7 in-memory cache/pub-sub cluster**, with end-to-end **Prometheus and Grafana observability**.

```
                                  +-----------------------+
                                  |   Client Applications |
                                  +-----------+-----------+
                                              |
                          (JWT / ?token= / Idempotency-Key)
                                              |
                                              v
                                  +-----------------------+
                                  |     Nginx Ingress     |
                                  |   - Load Balancer     |
                                  |   - SSE Stream Proxy  |
                                  |   - Token Redaction   |
                                  +-----+-----+-----+-----+
                                        |     |     |
                 +----------------------+     |     +----------------------+
                 v                            v                            v
     +-----------------------+    +-----------------------+    +-----------------------+
     |   QuickCart API #1    |    |   QuickCart API #2    |    |   QuickCart API #3    |
     | - Worker Pool         |    | - Worker Pool         |    | - Worker Pool         |
     | - Expiry Sweeper      |    | - Expiry Sweeper      |    | - Expiry Sweeper      |
     | - Local SSE Hub       |    | - Local SSE Hub       |    | - Local SSE Hub       |
     +-----------+-----------+    +-----------+-----------+    +-----------+-----------+
                 |                            |                            |
                 +---------------------+------+----------------------------+
                                       |
                   +-------------------+-------------------+
                   |                                       |
        (Cache / Rate Limit / PubSub)               (ACID Transactions / Locks)
                   v                                       v
       +-----------------------+               +-----------------------+
       |        Redis 7        |               |     PostgreSQL 16     |
       | - Cache-aside catalog |               | - Inventory FOR UPDATE|
       | - Sliding-window Lua  |               | - SKIP LOCKED sweep   |
       | - SSE Pub/Sub Hub     |               | - Atomic token rot.   |
       +-----------------------+               +-----------------------+
```

---

## Skills Demonstrated & Source Code Mapping

This project was built to demonstrate senior-level backend Go competencies required for quick-commerce infrastructure roles (e.g., Swiggy Instamart Associate SDE):

| Required Skill | Concrete Engineering Implementation | Key Source Files & Packages |
| :--- | :--- | :--- |
| **1. Idiomatic Go 1.25** | Clean package layout, domain interfaces, error wrapping with `%w`, zero reflection, module verification. | [`cmd/server/main.go`](cmd/server/main.go), [`internal/order/service.go`](internal/order/service.go), [`internal/config/config.go`](internal/config/config.go) |
| **2. Goroutines & Channels** | Real concurrent job dispatch, buffered channels, graceful worker draining, non-blocking broadcasts. | [`internal/queue/channel_queue.go`](internal/queue/channel_queue.go), [`internal/worker/worker_pool.go`](internal/worker/worker_pool.go), [`internal/tracking/sse_hub.go`](internal/tracking/sse_hub.go) |
| **3. Go Backend Development** | Production REST API with Gin, structured slog middleware, HMAC-SHA256 JWT auth, role-based access control (RBAC). | [`internal/auth/`](internal/auth/), [`internal/platform/middleware/`](internal/platform/middleware/), [`internal/cart/handler.go`](internal/cart/handler.go) |
| **4. Concurrency Patterns** | Bounded concurrency worker pool, fan-out/fan-in batch processing, context cancellation, sync.RWMutex. | [`internal/worker/batch.go`](internal/worker/batch.go), [`internal/inventory/expiry_worker.go`](internal/inventory/expiry_worker.go) |
| **5. Rigorous Testing** | Table-driven unit tests, repository fakes, race detector validation (`go test -race`), automated Testcontainers suites. | [`internal/order/race_test.go`](internal/order/race_test.go), [`internal/testutil/testcontainers.go`](internal/testutil/testcontainers.go), [`internal/tracking/sse_test.go`](internal/tracking/sse_test.go) |
| **6. Redis & Caching** | Cache-aside catalog lookups with TTL and invalidation, atomic Lua sliding-window rate limiting, Redis Pub/Sub SSE. | [`internal/platform/redis/cache.go`](internal/platform/redis/cache.go), [`internal/platform/redis/ratelimit.go`](internal/platform/redis/ratelimit.go) |
| **7. SQL & PostgreSQL** | Row-level locking (`FOR UPDATE`), deterministic lock ordering, `SKIP LOCKED` reservation expiry, atomic token rotation. | [`internal/order/repository.go`](internal/order/repository.go), [`internal/inventory/expiry_worker.go`](internal/inventory/expiry_worker.go), [`migrations/`](migrations/) |
| **8. Systems & Production Readiness** | 3-instance Nginx topology, bounded Prometheus metrics, auto-provisioned Grafana dashboards, embedded OpenAPI 3.0 docs, GitHub Actions CI. | [`deployments/docker-compose.yml`](deployments/docker-compose.yml), [`deployments/nginx/nginx.conf`](deployments/nginx/nginx.conf), [`.github/workflows/ci.yml`](.github/workflows/ci.yml), [`api/openapi.yaml`](api/openapi.yaml) |

---

## Reproducible Flash Sale Oversell Demo (1,000 Users vs 10 Items)

Under extreme flash-sale spikes, naive e-commerce engines oversell stock, resulting in costly fulfillment cancellations. QuickCart guarantees that **no matter how many concurrent requests hit the cluster, exactly 10 orders succeed and zero units are oversold**.

*(Note: Load tests represent simulated concurrent workloads executed against local container clusters).*

### Step 1: Boot the 3-Instance Cluster
```bash
docker compose -f deployments/docker-compose.yml up --build -d
```

### Step 2: Run the k6 Flash Sale Concurrency Scenario
Execute the automated 1,000 Virtual User flash sale test:
```bash
k6 run loadtest/03_oversell_headline.js
```

### Step 3: Inspect the Output
```text
✓ successful_orders............: 10     (Exactly 10 items purchased)
✓ inventory_conflicts..........: 990    (Exactly 990 requests rejected with HTTP 409)
✓ unexpected_failures..........: 0      (Zero 500 errors or deadlocks)
```

### Step 4: Verify Database Invariants Directly in PostgreSQL
Verify the absolute integrity of inventory and orders directly via `psql`:

```bash
docker exec -it quickcart-postgres psql -U quickcart -d quickcart -c "
  SELECT product_id, available, reserved
  FROM store_inventories
  WHERE store_id = 'a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d'
    AND product_id = 'c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f';
"
```
**Expected Output:**
```text
              product_id              | available | reserved 
--------------------------------------+-----------+----------
 c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f |         0 |       10
(1 row)
```

---

## Benchmark Results Summary

Full methodology, hardware specifications, and raw k6 outputs are documented in [docs/benchmarks.md](docs/benchmarks.md).

| Scenario | Tested Workload | Throughput (RPS) | Latency p95 | Error Rate | Verification Invariant |
| :--- | :--- | :---: | :---: | :---: | :--- |
| **Catalog Browsing** | 100 VUs read burst | ~1,850 RPS | 18.6 ms | 0.00% | Redis cache-aside hit ratio: 97.8% |
| **Cart & Checkout** | 50 VUs sustained checkouts | ~420 RPS | 82.5 ms | 0.00% | Full ACID transaction & payment |
| **Flash Sale Oversell**| **1,000 VUs simultaneous burst**| **~680 RPS** | **112.0 ms**| **0.00%** | **10 sold, 990 rejected (409 Conflict)** |
| **Live SSE Streams** | 50 VUs live tracking | ~210 RPS | 22.4 ms | 0.00% | Unbuffered chunked SSE via Nginx |

---

## Interactive API Documentation (OpenAPI 3.0 / Swagger UI)

Interactive documentation is served directly from the application binary using embedded OpenAPI specifications:

* **Swagger UI Interactive Explorer**: [`http://localhost:8080/docs`](http://localhost:8080/docs)
* **Raw OpenAPI 3.0 Specification**: [`http://localhost:8080/api/openapi.yaml`](http://localhost:8080/api/openapi.yaml)

### Key Endpoints

| Category | Method | Path | Auth / Role | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Health** | `GET` | `/healthz` | Public | Process liveness probe |
| **Health** | `GET` | `/readyz` | Public | Dependency readiness probe (Postgres + Redis) |
| **Metrics** | `GET` | `/metrics` | Bearer Token / Admin | Bounded Prometheus metrics |
| **Docs** | `GET` | `/docs` | Public | Interactive Swagger UI documentation |
| **Auth** | `POST` | `/api/v1/auth/register` | Public | Register customer account |
| **Auth** | `POST` | `/api/v1/auth/login` | Public | Login and obtain JWT token pair |
| **Auth** | `POST` | `/api/v1/auth/refresh` | Public | Atomic refresh token rotation |
| **Catalog** | `GET` | `/api/v1/stores` | Public | List active dark stores |
| **Catalog** | `GET` | `/api/v1/stores/:id/products` | Public | List products in dark store (Cached) |
| **Admin** | `POST` | `/api/v1/admin/stores/:id/inventory`| Admin Only | Upsert store inventory stock |
| **Cart** | `POST` | `/api/v1/cart/items` | Customer | Add product to shopping cart |
| **Orders** | `POST` | `/api/v1/checkout` | Customer | Atomic checkout with `Idempotency-Key` |
| **Payments**| `POST` | `/api/v1/orders/:id/pay` | Owner / Admin | Process payment with jittered backoff |
| **Payments**| `POST` | `/api/v1/payments/webhook` | HMAC Signature | Verified payment gateway callback |
| **Tracking**| `GET` | `/api/v1/orders/:id/events`| Owner / Admin | Real-time SSE status stream (`?token=`) |

---

## Quality Assurance & Testing

Run all validation suites locally:

```bash
# Verify Go module integrity
go mod verify

# Compile all application binaries
go build ./...

# Static analysis and linting
go vet ./...
golangci-lint run --timeout=5m

# Run complete test suite with Go Race Detector
go test -v -race ./...

# Run automated PostgreSQL 16 & Redis 7 Testcontainers integration suite
go test -v -race ./internal/testutil/...
```

---

## Production Cloud Deployment

Detailed step-by-step instructions for deploying QuickCart to cloud platforms (**Fly.io**, **AWS ECS Fargate**, and **Render**) with managed databases, secret management, and zero-downtime rolling upgrades are provided in [docs/deployment.md](docs/deployment.md).
