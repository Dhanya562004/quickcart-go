# QuickCart — System Architecture

QuickCart is engineered as a high-concurrency, distributed quick-commerce backend in **Go 1.25**, inspired by the operational architectures of **Swiggy Instamart, Blinkit, and Zepto**. It delivers strict sub-10-minute SLA order fulfillment, atomic inventory reservations with zero overselling, and live delivery tracking.

---

## 1. High-Level Distributed Topology

The production architecture consists of an **Nginx ingress load balancer** distributing client traffic across **three stateless Go API instances**, backed by a shared **PostgreSQL 16 relational database** and **Redis 7 in-memory cache/pub-sub cluster**, with integrated **Prometheus and Grafana observability**.

```mermaid
graph TD
    Client[Client Applications / Browsers] -->|HTTP / SSE / REST| Nginx[Nginx Ingress Reverse Proxy :8080]

    subgraph "Stateless Application Cluster"
        Nginx -->|Round-Robin| API1[QuickCart API Replica 1]
        Nginx -->|Round-Robin| API2[QuickCart API Replica 2]
        Nginx -->|Round-Robin| API3[QuickCart API Replica 3]
    end

    subgraph "Distributed Shared State"
        API1 -->|Cache-Aside / Rate Limit / PubSub| Redis[(Redis 7 Cluster)]
        API2 -->|Cache-Aside / Rate Limit / PubSub| Redis
        API3 -->|Cache-Aside / Rate Limit / PubSub| Redis

        API1 -->|ACID Transactions / Row Locks| Postgres[(PostgreSQL 16)]
        API2 -->|ACID Transactions / Row Locks| Postgres
        API3 -->|ACID Transactions / Row Locks| Postgres
    end

    subgraph "Background Concurrency Pipelines"
        API1 -.-> W1[Worker Pool: Confirmations & Drain]
        API2 -.-> W2[Worker Pool: Confirmations & Drain]
        API3 -.-> W3[Worker Pool: Confirmations & Drain]

        API1 -.-> E1[Expiry Reclaimer: SKIP LOCKED]
        API2 -.-> E2[Expiry Reclaimer: SKIP LOCKED]
        API3 -.-> E3[Expiry Reclaimer: SKIP LOCKED]
    end

    subgraph "Observability Platform"
        Prometheus[Prometheus :9090] -->|Scrape /metrics| API1
        Prometheus -->|Scrape /metrics| API2
        Prometheus -->|Scrape /metrics| API3
        Grafana[Grafana :3000] -->|Visualize Dashboards| Prometheus
    end
```

---

## 2. Distributed Concurrency & Inventory Guarantees

### Zero-Overselling Flash Sale Checkout

Under quick-commerce flash sales, thousands of users compete for limited items simultaneously. QuickCart achieves mathematical zero-overselling through deterministic row-level locking in PostgreSQL:

```mermaid
sequenceDiagram
    autonumber
    actor Customer as Authenticated Customer
    participant Nginx as Nginx Load Balancer
    participant API as QuickCart API (Replica 1/2/3)
    participant Redis as Redis 7 (Rate Limit)
    participant DB as PostgreSQL 16 (ACID)

    Customer->>Nginx: POST /api/v1/checkout (Idempotency-Key: UUID)
    Nginx->>API: Forward Request
    API->>Redis: EVALSHA slidingWindowLua(key="user:uid", window=1m, limit=100)
    Redis-->>API: Allowed (1), Remaining (99)

    API->>DB: BEGIN Transaction (Isolation: Read Committed)
    Note over API,DB: Deterministic Lock Sorting:<br/>Sort product IDs lexicographically to prevent deadlocks
    API->>DB: SELECT available, reserved FROM store_inventories<br/>WHERE store_id = $1 AND product_id IN ($2, $3)<br/>ORDER BY product_id FOR UPDATE

    alt Sufficient Inventory (available >= quantity)
        API->>DB: UPDATE store_inventories<br/>SET available = available - $qty, reserved = reserved + $qty<br/>WHERE store_id = $1 AND product_id = $2
        API->>DB: INSERT INTO orders (id, user_id, status='PLACED', idempotency_key=$key)
        API->>DB: INSERT INTO inventory_reservations (status='RESERVED', expires_at=now()+10m)
        API->>DB: COMMIT Transaction
        API-->>Nginx: 201 Created (Order DTO)
        Nginx-->>Customer: 201 Created
    else Insufficient Inventory (available < quantity)
        API->>DB: ROLLBACK Transaction
        API-->>Nginx: 409 Conflict (Product X is out of stock)
        Nginx-->>Customer: 409 Conflict
    end
```

---

## 3. Distributed Real-Time SSE Architecture

Browser `EventSource` connections stream live order status transitions. Because clients can connect to any of the 3 API instances behind Nginx, QuickCart uses **Redis Pub/Sub** to bridge instances:

```mermaid
sequenceDiagram
    autonumber
    actor ClientA as Customer A (Browser)
    participant Nginx as Nginx Ingress
    participant API1 as QuickCart API #1
    participant Redis as Redis Pub/Sub
    participant API2 as QuickCart API #2
    actor Dispatcher as Warehouse Worker

    ClientA->>Nginx: GET /api/v1/orders/ORD-123/events?token=<jwt>
    Nginx->>API1: Route connection (proxy_buffering off)
    API1->>API1: Verify JWT & Order Ownership
    API1->>API1: Register in-memory channel for ORD-123

    Note over Dispatcher,API2: Delivery transition occurs on API #2
    Dispatcher->>API2: POST /api/v1/orders/ORD-123/status (CONFIRMED)
    API2->>Redis: PUBLISH quickcart:order_events {order_id: ORD-123, status: CONFIRMED, instance_id: "api-2"}

    Redis->>API1: Dispatch message on channel quickcart:order_events
    API1->>API1: Check instance_id != "api-1" (Process foreign event)
    API1->>ClientA: SSE event: order_status {status: "CONFIRMED"}
```

---

## 4. Background Expiry Sweeps (`FOR UPDATE SKIP LOCKED`)

Unfulfilled reservations expire after 10 minutes (SLA window). The expiry worker reclaims abandoned stock without blocking concurrent checkout transactions:

```mermaid
graph LR
    Worker[Distributed Expiry Worker] -->|Every 10s| Query["SELECT * FROM inventory_reservations<br/>WHERE status = 'RESERVED' AND expires_at < NOW()<br/>FOR UPDATE SKIP LOCKED LIMIT 100"]
    Query --> Batch[Reclaim Stock in Store Inventories]
    Batch --> Release["UPDATE store_inventories<br/>SET available = available + qty, reserved = reserved - qty"]
    Release --> Cancel["UPDATE orders SET status = 'CANCELLED'"]
```

---

## 5. Security & Isolation Architecture

1. **Zero-Trust Identity**: Identity is never inferred from unverified client headers (`X-User-ID`, `X-User-Role`) or query parameters. Identity is strictly decoded from cryptographically verified HMAC-SHA256 JWT access tokens.
2. **Atomic Token Rotation**: Refresh tokens are stored as SHA-256 hashes in PostgreSQL. Single-use rotation uses row locks (`SELECT ... FOR UPDATE`) to eliminate race conditions and token replay attacks.
3. **Nginx Query Parameter Sanitization**: Nginx regex maps redact sensitive JWT tokens (`?token=eyJ...`) from HTTP access logs to prevent token leakage in log aggregation systems.
4. **Network Isolation**: PostgreSQL (`quickcart-db`) and Redis (`quickcart-redis`) are strictly isolated on internal Docker bridge networks and are unreachable from public networks.
