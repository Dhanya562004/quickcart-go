# Phase 1 Foundation: Why This Design (Interview Reference)

As an Associate SDE candidate for Swiggy Instamart, here is how to explain every technical and architectural decision made in Phase 1 during your system design and coding interviews.

---

## 1. Modular Monolith vs. Microservices

### Interview Question: "Why start QuickCart as a modular monolith rather than microservices?"
- **Operational Overhead & Latency**: Quick-commerce has strict 10-minute fulfillment SLAs. Splitting into microservices on Day 1 introduces distributed transactions (Saga / 2PC), network hops (10–20ms per internal RPC), network partitioning risks, and complex distributed tracing before product-market validation.
- **In-Process Concurrency**: In Go, in-memory communication between domain modules (e.g., catalog, inventory, order) happens via Go interfaces and channels with sub-microsecond latency and zero serialization overhead.
- **Clean Package Boundaries**: By enforcing strict package isolation under `internal/` (e.g., `internal/inventory`, `internal/order`) and exchanging domain models via Go interfaces, any module can be broken out into an independent microservice later without refactoring core business logic.

---

## 2. Zero-Overselling Inventory Reservation Architecture

### Interview Question: "How do you handle 1,000 customers buying the last 10 units at once without overselling?"
- **The Naive Approach (Pessimistic Row Locking)**:
  ```sql
  SELECT available_quantity FROM store_inventories WHERE store_id = $1 AND product_id = $2 FOR UPDATE;
  -- application check: if available >= requested { UPDATE ... }
  ```
  **Why it fails under high contention**:
  1. Serializes all 1,000 requests one after another.
  2. Acquires heavy exclusive row locks for the entire duration of the database transaction.
  3. Starves the connection pool (`pgxpool`), causing API timeouts.
  4. Prone to database deadlocks if two concurrent carts reserve the same items in opposite order.

- **The QuickCart Approach (Atomic Conditional Updates)**:
  ```sql
  UPDATE store_inventories
  SET 
      available_quantity = available_quantity - $3,
      reserved_quantity = reserved_quantity + $3,
      version = version + 1,
      updated_at = NOW()
  WHERE 
      store_id = $1 
      AND product_id = $2 
      AND available_quantity >= $3
  RETURNING store_id, product_id, available_quantity, reserved_quantity, version, updated_at;
  ```
  **Key Advantages**:
  1. **Atomic Tuple Evaluation**: PostgreSQL evaluates the predicate `available_quantity >= $3` atomically during row update.
  2. **Zero Row-Lock Duration**: The lock is held only for the microsecond duration of the single UPDATE tuple write.
  3. **Immediate Failure Signal**: If stock is depleted, the statement updates 0 rows, and `pgx` returns `pgx.ErrNoRows`. The application immediately returns `ErrInsufficientStock` without holding open transactions.
  4. **Lock Ordering**: Prevents lock-ordering deadlocks when all transactions sort items by `product_id` before acquiring reservations.

---

## 3. Inventory Reservation Lifecycle State Machine

### Transitions Guarded by `WHERE status = 'RESERVED'`:
```text
                  ┌──────────────────────┐
                  │       RESERVED       │
                  │   (TTL: 10 minutes)  │
                  └──────────┬───────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
     Payment Success   Payment Failed   TTL Expired (Worker)
            ▼                ▼                ▼
     ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
     │  CONFIRMED  │  │  RELEASED   │  │   EXPIRED   │
     └─────────────┘  └─────────────┘  └─────────────┘
```

1. **Atomic Insertion**: When checkout begins, the order and reservations are written in the same database transaction.
2. **State Protection**: Transitions from `RESERVED` to `CONFIRMED`, `RELEASED`, or `EXPIRED` require `WHERE status = 'RESERVED'`. This guarantees idempotency and prevents race conditions where an expired reservation is accidentally confirmed.
3. **Background Expiry Worker with `FOR UPDATE SKIP LOCKED`**:
   ```sql
   SELECT id, order_id, store_id, product_id, quantity, status, expires_at
   FROM inventory_reservations
   WHERE status = 'RESERVED' AND expires_at <= NOW()
   ORDER BY expires_at ASC
   LIMIT $1
   FOR UPDATE SKIP LOCKED;
   ```
   Multiple background worker instances can safely scan and process expired reservations in parallel without lock contention or duplicate releases.

---

## 4. Connection Pooling (`pgxpool.Pool`)

### Interview Question: "Why pgx instead of database/sql, and how do you size your connection pool?"
- **Driver Performance**: `pgx/v5` outperforms the standard `database/sql` driver by utilizing PostgreSQL binary format encoding/decoding, native prepared statement pipelines, and native types.
- **Sizing Formula**: In high-concurrency Go services, creating unbounded connections exhausts PostgreSQL server memory. We configure:
  - `DBMaxConns = 25` (tuned to PostgreSQL server CPU cores: `conns = (2 * cores) + disk_spindle_count`)
  - `DBMinConns = 5` (avoids latency spikes from cold connection establishment)
  - `DBMaxConnLifetime = 1h` (recycles connections to avoid backend memory leaks or stale TCP sockets)
  - `DBMaxConnIdleTime = 30m`

---

## 5. Graceful Shutdown & Context Cancellation

### Interview Question: "What happens to active customer requests when a QuickCart pod deploys or terminates?"
1. The service traps `SIGINT` and `SIGTERM` OS signals via a buffered signal channel.
2. Upon receiving a signal, the server stops accepting new incoming HTTP connections (`srv.Shutdown(ctx)`).
3. A deadline context `context.WithTimeout(context.Background(), cfg.ShutdownTimeout)` (default 5s) gives all in-flight checkout and reservation transactions time to complete cleanly.
4. Active database connections are drained safely with `dbConn.Close()`.
5. No client encounters a connection reset (`ECONNRESET`) or partial transaction write.
