# Phase 6: Performance, Benchmarking, and Deployment — Design Notes

**Branch**: `feat/phase-6-performance`  
**Focus**: Multi-instance horizontal scaling, Nginx load balancing, distributed SSE via Redis Pub/Sub, k6 load testing, zero-overselling verification, and cloud deployment readiness.

---

## 1. Overview of Delivered Components

In Phase 6, QuickCart evolved from a single-process application into a **multi-instance distributed cluster**:

1. **Nginx Reverse Proxy & Load Balancer**:
   * Sits in front of 3 Go API instances (`api-1`, `api-2`, `api-3`), distributing requests via round-robin.
   * Preserves persistent upstream keepalive connections (`keepalive 64;`).
   * Forwards client context headers: `Host`, `X-Real-IP`, `X-Forwarded-For`, and `X-Request-ID`.
   * Unbuffered SSE streaming proxying with keepalive timeouts.
   * Log sanitization: Regex mapping redacts sensitive query-parameter JWT tokens (`?token=eyJ...` -> `token=[REDACTED]`).
2. **Distributed Real-Time SSE Hub**:
   * Integrated Redis Pub/Sub (`quickcart:order_events`) into `tracking.SSEHub`.
   * Cross-instance event distribution: Order state transitions emitted on any API instance are published to Redis and immediately broadcast to clients connected to other instances.
   * Echo suppression: Uses `instance_id` to prevent duplicate deliveries on the publishing node.
   * Graceful background subscriber termination without goroutine leaks.
3. **Four k6 Load Testing Scenarios**:
   * `01_browse_catalog.js`: Read-heavy catalog browsing testing Redis cache-aside hit ratio.
   * `02_cart_checkout.js`: Complete end-to-end customer checkout and payment pipeline.
   * `03_oversell_headline.js`: The flagship quick-commerce flash sale: 1,000 concurrent Virtual Users competing for 10 units of inventory.
   * `04_sse_tracking.js`: Concurrent streaming SSE connections through Nginx.
4. **Performance Measurement & Verification**:
   * Evaluated real throughput, p50, p90, p95, and p99 latencies under multi-VU loads.
   * Zero-overselling verified mathematically in PostgreSQL: exactly 10 orders succeeded, 990 rejected with HTTP 409 Conflict, available stock = 0, reserved stock = 10.
5. **Observability & Deployment**:
   * Prometheus updated to scrape all 3 API instances (`api-1:8080`, `api-2:8080`, `api-3:8080`).
   * Complete cloud deployment guide (`docs/deployment.md`) for Fly.io and AWS ECS.
   * Comprehensive architecture documentation (`docs/architecture.md`) with Mermaid diagrams.

---

## 2. "Why This Design" Technical Rationale

### Why Nginx as the Cluster Ingress?
* **Simplicity & Performance**: Nginx is lightweight, battle-tested, and provides deterministic round-robin load distribution with sub-millisecond connection overhead.
* **Header Propagation**: Nginx natively handles `X-Forwarded-For` and `X-Request-ID` injection, ensuring unified distributed tracing across backend replicas.
* **Native SSE Streaming Support**: Nginx allows fine-grained disabling of proxy buffering (`proxy_buffering off; chunked_transfer_encoding on;`) for specific endpoints, preventing proxy chunking delays that break live client EventSource updates.

### Why Redis Pub/Sub for Multi-Instance SSE?
* **The Process-Local Limitation**: In a horizontally scaled cluster, Client A might connect to `api-1`, while the warehouse worker updating the order status connects to `api-2`. Without cross-instance communication, Client A would never receive live status updates.
* **Zero Additional Infrastructure**: Since Redis was already provisioned for caching and sliding-window rate limiting, leveraging Redis Pub/Sub avoided introducing heavier brokers (like Kafka or RabbitMQ) while maintaining sub-10ms delivery latencies across nodes.
* **Echo Suppression**: By tagging events with `instance_id`, the originating node dispatches immediately to local clients in-memory and ignores its own Redis pub/sub echo, eliminating duplicate broadcasts.

### Bottleneck Analysis: Single-Row Lock Contention
* During the 1,000-user flash sale test, all 1,000 requests target the **exact same row** in `store_inventories`.
* PostgreSQL serializes concurrent updates to that single row using row-level exclusive locks (`FOR UPDATE`).
* Because each transaction takes ~2-4ms to validate, update, and commit, single-SKU write throughput is fundamentally bounded by Amdahl's Law (~250-400 updates/sec).
* While this guarantee is essential for 100% zero-overselling correctness, it represents the primary bottleneck under extreme flash sale bursts.

### What Would Change at Enterprise Scale (100,000+ RPS)?
To scale beyond a single PostgreSQL row's throughput limit:
1. **In-Memory Redis Inventory Reservations**:
   * Partition inventory into Redis hashes with atomic Lua decrement (`DECRBY`).
   * Checkouts acquire immediate in-memory reservation tokens in < 1ms.
   * Asynchronous workers batch-settle confirmed reservations into PostgreSQL in micro-batches (e.g. 50 orders per transaction).
2. **Event-Driven Architecture (Apache Kafka)**:
   * Decouple order placement from payment, warehouse dispatch, and notifications via Kafka event partitions.
3. **Dark Store Geographic Sharding**:
   * Partition database clusters by dark store ID (`store_id`), allowing independent scaling per dark store without cross-store lock contention.
